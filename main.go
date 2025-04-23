package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"

	"github.com/creack/pty"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/term"
)

// Configuration options
type Config struct {
	ClaudeCommand    string             // Command to start the Claude CLI
	ClaudeArgs       []string           // Arguments for Claude CLI
	RootDirectory    string             // Directory to watch for changes
	AICommentPattern *regexp.Regexp     // Pattern to detect AI comments
	PromptTemplate   *template.Template // Template for the prompt when a file changes
	Debug            bool               // Enable debug output
}

// Default prompt template
const DefaultPromptTemplate = "Read the file at {{.File}}. Any comments in this file that end in `ai!` are instructions for you to modify this file. For the scope of this instruction, you are not permitted to modify other files as part of the instructions in these comments. In other words, in response to this prompt, you are only permitted to modify the file at path {{.File}}. Once you make the requested modifications, remove the comment that instructed you."

// Template data structure
type TemplateData struct {
	File string // Absolute path of the file that changed
}

// Helper function to print debug messages
func debugLog(config *Config, format string, args ...interface{}) {
	if config.Debug {
		fmt.Fprintf(os.Stderr, "Debug: "+format+"\n", args...)
	}
}

// printHelp displays the usage information
func printHelp() {
	fmt.Println("Usage: claudewatch [options] [directory] [-- claude_arguments]")
	fmt.Println("")
	fmt.Println("A transparent wrapper for the Claude CLI that watches file changes and")
	fmt.Println("automatically sends AI-directed instructions to Claude.")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -h, --help       Show this help message and exit")
	fmt.Println("  --debug          Enable debug output")
	fmt.Println("  --prompt TEXT    Customize the prompt template (use {{.File}} as a variable)")
	fmt.Println("  --               Everything after this marker is passed directly to Claude")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  claudewatch                   # Watch current directory")
	fmt.Println("  claudewatch /path/to/project  # Watch specific directory")
	fmt.Println("  claudewatch -- --model-name claude-3-opus-20240229")
	fmt.Println("")
	fmt.Println("For more information, see: https://github.com/jtrim/claudewatch")
	os.Exit(0)
}

func main() {
	// Check for help flag
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" || arg == "help" {
			printHelp()
		}
	}

	// Parse the default prompt template
	tmpl, err := template.New("prompt").Parse(DefaultPromptTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing default prompt template: %v\n", err)
		os.Exit(1)
	}

	// Set initial configuration
	config := Config{
		ClaudeCommand:    "claude",
		ClaudeArgs:       []string{},
		RootDirectory:    ".",
		AICommentPattern: regexp.MustCompile(`(?m)(?:\s*\/\/|\s*#|\s*\/\*|\s*\*)\s*.*ai!`),
		PromptTemplate:   tmpl,
		Debug:            false, // Debug mode off by default
	}

	// Starting message that will only be shown in debug mode
	debugLog(&config, "Starting claudewatch...")

	// Parse command line arguments
	args := os.Args[1:]
	var claudeArgs []string
	watchDirSpecified := false

	// Process arguments
	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Check for "--" separator (everything after goes to Claude)
		if arg == "--" {
			if i+1 < len(args) {
				claudeArgs = args[i+1:]
			}
			break
		}

		// Check for --debug flag
		if arg == "--debug" {
			config.Debug = true
			debugLog(&config, "Debug mode enabled")
			continue
		}

		// Check for --prompt flag
		if arg == "--prompt" {
			if i+1 < len(args) {
				customTemplate := args[i+1]
				tmpl, err := template.New("prompt").Parse(customTemplate)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error parsing custom prompt template: %v\n", err)
					os.Exit(1)
				}
				config.PromptTemplate = tmpl
				debugLog(&config, "Using custom prompt template: %s", customTemplate)
				i++ // Skip the next argument (the template)
				continue
			}
		}

		// Check if arg is a directory to watch
		if !watchDirSpecified {
			fileInfo, err := os.Stat(arg)
			if err == nil && fileInfo.IsDir() {
				config.RootDirectory = arg
				watchDirSpecified = true
				debugLog(&config, "Watching directory: %s", config.RootDirectory)
				continue
			}
		}

		// If we get here, this is an argument to pass to Claude
		claudeArgs = append(claudeArgs, arg)
	}

	// Set Claude arguments
	config.ClaudeArgs = claudeArgs
	if len(claudeArgs) > 0 {
		debugLog(&config, "Passing arguments to Claude: %v", config.ClaudeArgs)
	}

	// Create a new file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating file watcher: %v\n", err)
		os.Exit(1)
	}
	defer watcher.Close()

	// Add the root directory explicitly to watch
	err = watcher.Add(config.RootDirectory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error watching root directory %s: %v\n", config.RootDirectory, err)
	} else {
		debugLog(&config, "Watching root directory: %s", config.RootDirectory)
	}

	// Debug: Check if Claude executable exists
	path, err := exec.LookPath(config.ClaudeCommand)
	if err != nil {
		debugLog(&config, "Claude command not found in PATH: %v", err)
		debugLog(&config, "Searching for claude-cli or anthropic alternatives...")

		// Try alternative names
		alternatives := []string{"claude-cli", "anthropic", "anthropic-cli"}
		for _, alt := range alternatives {
			path, err = exec.LookPath(alt)
			if err == nil {
				debugLog(&config, "Found alternative command: %s", alt)
				config.ClaudeCommand = alt
				break
			}
		}
	} else {
		debugLog(&config, "Claude found at path: %s", path)
	}

	// Create a channel for file change prompts
	promptChan := make(chan string)

	// Start Claude process with PTY
	debugLog(&config, "Starting Claude with command: %s %v using PTY", config.ClaudeCommand, config.ClaudeArgs)
	claudeCmd := exec.Command(config.ClaudeCommand, config.ClaudeArgs...)

	// Start the command with a pty
	ptyMaster, err := pty.Start(claudeCmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting Claude with PTY: %v\n", err)
		os.Exit(1)
	}
	// Make sure to close the pty at the end
	defer ptyMaster.Close()

	// Handle pty size
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptyMaster); err != nil {
				fmt.Fprintf(os.Stderr, "Error resizing pty: %s\n", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH                        // Initial resize
	defer func() { signal.Stop(ch); close(ch) }() // Cleanup signals when done

	// Set stdin in raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting terminal to raw mode: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }() // Best effort

	// Create waitgroup to manage goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine to copy stdin to the pty and the pty to stdout
	go func() {
		defer wg.Done()
		// Copy stdin to the pty
		go func() { io.Copy(ptyMaster, os.Stdin) }()
		// Copy the pty to stdout
		io.Copy(os.Stdout, ptyMaster)
	}()

	// Goroutine to handle file change prompts
	go func() {
		defer wg.Done()

		// Start the file watcher
		processedFiles := make(map[string]time.Time)

		// Monitor files for changes
		go func() {
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}

					// Only process write events for regular files
					if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
						// Check if it's a file we should process
						fileInfo, err := os.Stat(event.Name)
						if err != nil {
							continue
						}

						// Skip directories and special files
						if fileInfo.IsDir() ||
							strings.HasPrefix(filepath.Base(event.Name), ".") ||
							isEmacsTemp(filepath.Base(event.Name)) {
							continue
						}

						// Skip files processed recently
						now := time.Now()
						if lastProcessed, exists := processedFiles[event.Name]; exists {
							if now.Sub(lastProcessed) < time.Second {
								continue
							}
						}
						processedFiles[event.Name] = now

						// Check if file contains AI comments
						content, err := os.ReadFile(event.Name)
						if err != nil {
							continue
						}

						if config.AICommentPattern.Match(content) {
							absPath, err := filepath.Abs(event.Name)
							if err != nil {
								continue
							}

							// Log file change
							fmt.Fprintf(os.Stderr, "\r\n[File change detected: %s - sending to Claude]\r\n", event.Name)

							// Prepare the template data
							data := TemplateData{
								File: absPath,
							}

							// Execute the template
							var promptBuf strings.Builder
							err = config.PromptTemplate.Execute(&promptBuf, data)
							if err != nil {
								fmt.Fprintf(os.Stderr, "Error executing prompt template: %v\n", err)
								continue
							}

							// Send the generated prompt to the channel for processing
							promptChan <- promptBuf.String()
						}
					}

					// If a directory is created, watch it
					if event.Has(fsnotify.Create) {
						fileInfo, err := os.Stat(event.Name)
						if err == nil && fileInfo.IsDir() && !strings.HasPrefix(filepath.Base(event.Name), ".") {
							watcher.Add(event.Name)
						}
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				}
			}
		}()

		// Process prompts from file changes
		for prompt := range promptChan {
			// Write prompt to Claude's stdin
			debugLog(&config, "Writing prompt to Claude's PTY")
			_, err := ptyMaster.Write([]byte(prompt))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing prompt to Claude's PTY: %v\r\n", err)
			}

			// Add a delay to ensure prompt is fully processed
			time.Sleep(300 * time.Millisecond)

			// Try just Carriage Return (ASCII 13)
			debugLog(&config, "Sending Carriage Return (ASCII 13) only")
			_, err = ptyMaster.Write([]byte{13})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error sending CR to Claude's PTY: %v\r\n", err)
			}
		}
	}()

	// Recursively add directories to watch
	err = filepath.Walk(config.RootDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip hidden directories and git directories
			if strings.HasPrefix(info.Name(), ".") {
				debugLog(&config, "Skipping hidden directory: %s", path)
				return filepath.SkipDir
			}

			// Skip .git directories
			if info.Name() == ".git" || strings.Contains(path, "/.git/") {
				debugLog(&config, "Skipping git directory: %s", path)
				return filepath.SkipDir
			}

			err = watcher.Add(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error watching directory %s: %v\n", path, err)
			} else {
				debugLog(&config, "Watching directory: %s", path)
			}
		}
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directories: %v\n", err)
	}

	// Wait for Claude to finish
	err = claudeCmd.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Claude process ended with error: %v\n", err)
	}

	// Close the prompt channel and wait for goroutines to finish
	close(promptChan)
	wg.Wait()
}

// isEmacsTemp checks if a filename is an Emacs temporary file
func isEmacsTemp(filename string) bool {
	// Emacs auto-save files: #filename#
	if strings.HasPrefix(filename, "#") && strings.HasSuffix(filename, "#") {
		return true
	}

	// Emacs backup files: filename~
	if strings.HasSuffix(filename, "~") {
		return true
	}

	// Emacs lock files: .#filename
	if strings.HasPrefix(filename, ".#") {
		return true
	}

	return false
}
