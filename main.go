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
	RootDirectories  []string           // Directories to watch for changes
	AICommentPattern *regexp.Regexp     // Pattern to detect AI comments
	PromptTemplate   *template.Template // Template for the prompt when a file changes
	IgnorePattern    *regexp.Regexp     // Pattern to ignore files when watching
	IgnorePatterns   IgnorePatterns     // Patterns from .claudewatchignore file
	Debug            bool               // Enable debug output
	DebugOut         io.Writer          // Destination for debug output (.claudewatchdebug)
	DebugPath        string             // Absolute path of the debug output file
	ErrorOut         io.Writer          // Destination for always-on status/error output (.claudewatcherror)
	ErrorPath        string             // Absolute path of the error output file
}

// GetDefaultPromptTemplate returns the default template for prompts ai:ignore
func GetDefaultPromptTemplate() (*template.Template, error) {
	templateText := `Modify {{.File}}. Address the feedback in the following comments:

{{range .Markers}}Line {{.LineNumber}}: {{.LineText}}
{{end}}
For the scope of this instruction, do not modify any other files. However, if modifying other files would be necessary to fully address the feedback, stop, explain your reasoning, and wait for further instruction.

Once your editing task is complete, stop and await instruction.`

	return template.New("prompt").Parse(templateText)
}

// loadPromptTemplate reads and parses a .claudewatchprompt file.
func loadPromptTemplate(path string) (*template.Template, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return template.New("prompt").Parse(string(content))
}

// promptResolver picks the prompt template for a changed file. Unless a prompt
// was supplied explicitly (override), it finds the nearest .claudewatchprompt to
// the file's directory, caching the result per directory so the filesystem walk
// happens at most once per directory.
type promptResolver struct {
	defaultTmpl *template.Template
	override    *template.Template
	debugOut    io.Writer
	mu          sync.Mutex
	cache       map[string]*template.Template
}

func newPromptResolver(defaultTmpl, override *template.Template, debugOut io.Writer) *promptResolver {
	return &promptResolver{
		defaultTmpl: defaultTmpl,
		override:    override,
		debugOut:    debugOut,
		cache:       make(map[string]*template.Template),
	}
}

// resolve returns the prompt template to use for the file at filePath.
func (r *promptResolver) resolve(filePath string) *template.Template {
	if r.override != nil {
		return r.override
	}

	dir := filepath.Dir(filePath)

	r.mu.Lock()
	defer r.mu.Unlock()

	if cached, ok := r.cache[dir]; ok {
		return cached
	}

	tmpl := r.defaultTmpl
	if promptPath := findPromptFile(dir); promptPath != "" {
		if parsed, err := loadPromptTemplate(promptPath); err == nil {
			tmpl = parsed
			if r.debugOut != nil {
				fmt.Fprintf(r.debugOut, "Debug: using prompt template from %s for %s\n", promptPath, dir)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Warning: ignoring unparseable prompt file %s: %v\n", promptPath, err)
		}
	} else if r.debugOut != nil {
		fmt.Fprintf(r.debugOut, "Debug: no .claudewatchprompt found for %s, using default prompt\n", dir)
	}

	r.cache[dir] = tmpl
	return tmpl
}

// Template data structure
type TemplateData struct {
	File    string             // Absolute path of the file that changed
	Markers []AIMarkerLocation // Locations of AI markers with line numbers
}

// Helper function to print debug messages
func debugLog(config *Config, format string, args ...interface{}) {
	if config.Debug && config.DebugOut != nil {
		fmt.Fprintf(config.DebugOut, "Debug: "+format+"\n", args...)
	}
}

// errorLog prints always-on status/error messages to .claudewatcherror.
// These must never go to the live terminal: once Claude's PTY-based TUI owns
// it, a direct write from claudewatch would corrupt its rendering.
func errorLog(config *Config, format string, args ...interface{}) {
	if config.ErrorOut != nil {
		fmt.Fprintf(config.ErrorOut, format+"\n", args...)
	}
}

// printHelp displays the usage information
func printHelp() {
	fmt.Println("Usage: claudewatch [options] [directory...] [-- claude_arguments]")
	fmt.Println("")
	fmt.Println("A transparent wrapper for the Claude CLI that watches file changes and")
	fmt.Println("automatically sends AI-directed instructions to Claude.")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -h, --help       Show this help message and exit")
	fmt.Println("  --debug          Enable debug output (appended to .claudewatchdebug in the current directory)")
	fmt.Println("  --prompt TEXT    Customize the prompt template (use {{.File}} for file path and {{.Markers}} for the detected markers with line numbers)")
	fmt.Println("  --ignore REGEX   Ignore files matching this regex pattern when watching")
	fmt.Println("  --               Everything after this marker is passed directly to Claude")
	fmt.Println("")
	fmt.Println("Features:")
	fmt.Println("  - Add '" + strings.Join(supportedAIMarkers, "', '") + "' at the end of a comment to trigger Claude to process that instruction") // ai:ignore
	fmt.Println("  - Add 'ai:ignore' in a comment line before or on the same line as an instruction marker to skip processing it")                  // ai:ignore
	fmt.Println("  - Create a .claudewatchignore file with one regex pattern per line to exclude files from being watched")
	fmt.Println("  - Place a .claudewatchprompt file at or above the run directory to override the default prompt (nearest wins; --prompt still takes precedence)")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  claudewatch                   # Watch current directory")
	fmt.Println("  claudewatch /path/to/project  # Watch specific directory")
	fmt.Println("  claudewatch dir1 dir2         # Watch multiple directories")
	fmt.Println("  claudewatch --ignore \"\\.js$\" # Ignore all .js files")
	fmt.Println("  claudewatch -- --model-name claude-3-opus-20240229")
	fmt.Println("")
	fmt.Println("For more information, see: https://github.com/jtrim/claudewatch")
	os.Exit(0)
}

// watchDirectory adds a directory and its subdirectories to the watcher
// Returns true if the directory was added, false if it was skipped
func watchDirectory(watcher *fsnotify.Watcher, dirPath string, config *Config, skipRoot bool) error {
	debugLog(config, "Considering path for watching: %s", dirPath)

	// Get directory info
	info, err := os.Stat(dirPath)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return nil
	}

	// Root directory check
	name := info.Name()

	// Skip hidden directories (but not . or .. directory references)
	if IsHiddenOrSpecialFile(dirPath) {
		debugLog(config, "Skipping hidden directory: %s", dirPath)
		return filepath.SkipDir
	}

	// Skip .git directories
	if name == ".git" || strings.Contains(dirPath, "/.git/") {
		debugLog(config, "Skipping git directory: %s", dirPath)
		return filepath.SkipDir
	}

	// Check if directory should be ignored based on patterns
	if shouldIgnore, reason := ShouldIgnorePathWithConfig(dirPath, config); shouldIgnore {
		debugLog(config, "Skipping directory due to %s: %s", reason, dirPath)
		return filepath.SkipDir
	}

	// Add the directory to the watcher if not skipping root
	if !skipRoot {
		err = watcher.Add(dirPath)
		if err != nil {
			errorLog(config, "Error watching directory %s: %v", dirPath, err)
		} else {
			debugLog(config, "Watching directory: %s", dirPath)
		}
	}

	// Walk subdirectories
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory (already processed)
		if path == dirPath {
			return nil
		}

		if !info.IsDir() {
			return nil
		}

		// Skip hidden directories
		if IsHiddenOrSpecialFile(path) {
			debugLog(config, "Skipping hidden subdirectory: %s", path)
			return filepath.SkipDir
		}

		// Skip .git directories
		if info.Name() == ".git" || strings.Contains(path, "/.git/") {
			debugLog(config, "Skipping git subdirectory: %s", path)
			return filepath.SkipDir
		}

		// Check if subdirectory should be ignored
		if shouldIgnore, reason := ShouldIgnorePathWithConfig(path, config); shouldIgnore {
			debugLog(config, "Skipping subdirectory due to %s: %s", reason, path)
			return filepath.SkipDir
		}

		// Add the subdirectory to the watcher
		err = watcher.Add(path)
		if err != nil {
			debugLog(config, "Error watching subdirectory %s: %v", path, err)
		} else {
			debugLog(config, "Watching subdirectory: %s", path)
		}

		return nil
	})

	return err
}

func main() {
	// Check for help flag
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" || arg == "help" {
			printHelp()
		}
	}

	// Get the default prompt template
	tmpl, err := GetDefaultPromptTemplate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing default prompt template: %v\n", err)
		os.Exit(1)
	}

	// Set initial configuration
	config := Config{
		ClaudeCommand:    "claude",
		ClaudeArgs:       []string{},
		RootDirectories:  nil,
		AICommentPattern: markerPattern, // Using pattern from util.go
		PromptTemplate:   tmpl,
		IgnorePattern:    nil,   // Default to not ignoring any files
		IgnorePatterns:   nil,   // Will be loaded from .claudewatchignore
		Debug:            false, // Debug mode off by default
	}

	// Detect --debug up front (before the full parse) so diagnostics from
	// argument parsing are captured too. When set, append them to a
	// .claudewatchdebug file in the current directory instead of the terminal,
	// where Claude's full-screen TUI would otherwise clobber them.
	for _, arg := range os.Args[1:] {
		if arg == "--" {
			break
		}
		if arg == "--debug" {
			config.Debug = true
			break
		}
	}
	if config.Debug {
		debugPath, absErr := filepath.Abs(".claudewatchdebug")
		if absErr != nil {
			debugPath = ".claudewatchdebug"
		}
		debugFile, openErr := os.OpenFile(debugPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if openErr != nil {
			fmt.Fprintf(os.Stderr, "Error opening debug log %s: %v\n", debugPath, openErr)
			os.Exit(1)
		}
		defer debugFile.Close()
		config.DebugOut = debugFile
		config.DebugPath = debugPath
		fmt.Fprintf(debugFile, "\n=== claudewatch debug session started %s ===\n", time.Now().Format(time.RFC3339))
		fmt.Fprintf(os.Stderr, "claudewatch: --debug enabled, appending debug output to %s\n", debugPath)
	}

	// Always open .claudewatcherror for status/error output that must
	// survive regardless of --debug: once Claude's PTY-based TUI owns the
	// terminal, writing there directly would corrupt its rendering.
	errorPath, errAbsErr := filepath.Abs(".claudewatcherror")
	if errAbsErr != nil {
		errorPath = ".claudewatcherror"
	}
	errorFile, errOpenErr := os.OpenFile(errorPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if errOpenErr != nil {
		fmt.Fprintf(os.Stderr, "Error opening error log %s: %v\n", errorPath, errOpenErr)
		os.Exit(1)
	}
	defer errorFile.Close()
	config.ErrorOut = errorFile
	config.ErrorPath = errorPath
	fmt.Fprintf(errorFile, "\n=== claudewatch session started %s ===\n", time.Now().Format(time.RFC3339))

	// Starting message that will only be shown in debug mode
	debugLog(&config, "Starting claudewatch...")

	// Parse command line arguments
	args := os.Args[1:]
	var claudeArgs []string
	promptFromFlag := false

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
				promptFromFlag = true
				debugLog(&config, "Using custom prompt template: %s", customTemplate)
				debugLog(&config, "Note: Make sure your template contains {{.Markers}} for line numbers")
				i++ // Skip the next argument (the template)
				continue
			}
		}

		// Check for --ignore flag
		if arg == "--ignore" {
			if i+1 < len(args) {
				ignorePattern := args[i+1]
				pattern, err := regexp.Compile(ignorePattern)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error parsing ignore pattern: %v\n", err)
					os.Exit(1)
				}
				config.IgnorePattern = pattern
				debugLog(&config, "Using ignore pattern: %s", ignorePattern)
				i++ // Skip the next argument (the pattern)
				continue
			}
		}

		// Check if arg is a directory to watch (multiple directories allowed)
		if fileInfo, statErr := os.Stat(arg); statErr == nil && fileInfo.IsDir() {
			config.RootDirectories = append(config.RootDirectories, arg)
			debugLog(&config, "Watching directory: %s", arg)
			continue
		}

		// If we get here, this is an argument to pass to Claude
		claudeArgs = append(claudeArgs, arg)
	}

	// Set Claude arguments
	config.ClaudeArgs = claudeArgs
	if len(claudeArgs) > 0 {
		debugLog(&config, "Passing arguments to Claude: %v", config.ClaudeArgs)
	}

	// Default to watching the current directory if none were specified
	if len(config.RootDirectories) == 0 {
		config.RootDirectories = []string{"."}
	}

	// Build the prompt resolver. When --prompt is given it wins for every file;
	// otherwise the nearest .claudewatchprompt to each changed file is used,
	// discovered per change and cached per directory.
	var promptOverride *template.Template
	if promptFromFlag {
		promptOverride = config.PromptTemplate
	}
	resolver := newPromptResolver(config.PromptTemplate, promptOverride, config.DebugOut)

	// Load ignore patterns from .claudewatchignore in each watched root
	for _, root := range config.RootDirectories {
		ignorePatterns, loadErr := LoadIgnorePatterns(root)
		if loadErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error loading .claudewatchignore in %s: %v\n", root, loadErr)
			continue
		}
		if ignorePatterns != nil {
			config.IgnorePatterns = append(config.IgnorePatterns, ignorePatterns...)
			debugLog(&config, "Loaded %d patterns from %s/.claudewatchignore", len(ignorePatterns), root)
		}
	}

	// Create a new file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating file watcher: %v\n", err)
		os.Exit(1)
	}
	defer watcher.Close()

	// Recursively add all directories to watch from each root
	for _, root := range config.RootDirectories {
		debugLog(&config, "Setting up recursive file watching from root: %s", root)
		if watchErr := watchDirectory(watcher, root, &config, false); watchErr != nil {
			fmt.Fprintf(os.Stderr, "Error setting up recursive file watching for %s: %v\n", root, watchErr)
		}
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

	// claudeDone is closed once Claude exits, so anything still trying to
	// write into the PTY (in particular the prompt injector) can stop
	// instead of repeatedly failing against a dead process.
	claudeDone := make(chan struct{})
	var claudeWaitErr error
	go func() {
		claudeWaitErr = claudeCmd.Wait()
		close(claudeDone)
	}()

	// Handle pty size
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptyMaster); err != nil {
				errorLog(&config, "Error resizing pty: %s", err)
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

	// syncedPty serializes writes into the PTY: the human's forwarded
	// keystrokes and injected marker-fix prompts are two independent write
	// sources, and without this they could interleave at the byte level.
	syncedPty := &syncWriter{w: ptyMaster}

	// pasteDetector watches Claude's output for the bracketed-paste enable
	// sequence, so injected prompts can be wrapped the same way a real
	// paste would be reported (see injector.go).
	pasteDetector := &pasteModeDetector{}

	// Goroutine to copy stdin to the pty and the pty to stdout
	go func() {
		defer wg.Done()
		// Copy stdin to the pty
		go func() { io.Copy(syncedPty, os.Stdin) }()
		// Copy the pty to stdout, watching for the bracketed-paste enable
		// sequence along the way.
		if err := copyAndDetectPaste(os.Stdout, ptyMaster, pasteDetector); err != nil {
			errorLog(&config, "Error copying Claude output: %v", err)
		}
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

					// Never react to writes to our own debug/error logs, and
					// never log this skip either: logging it would write to
					// one of those files, triggering another event and
					// looping forever. This check must stay first, before
					// any debugLog call in this case.
					if config.DebugPath != "" || config.ErrorPath != "" {
						if abs, absErr := filepath.Abs(event.Name); absErr == nil {
							if abs == config.DebugPath || abs == config.ErrorPath {
								continue
							}
						}
					}

					debugLog(&config, "Received event: %s (op: %s)", event.Name, event.Op)

					// Process write events and create events
					if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
						// Check if the file/directory exists
						fileInfo, err := os.Stat(event.Name)
						if err != nil {
							continue
						}

						// Handle directory creation separately
						if fileInfo.IsDir() && event.Has(fsnotify.Create) {
							debugLog(&config, "New directory created: %s", event.Name)

							// Try to watch the new directory and its subdirectories
							err = watchDirectory(watcher, event.Name, &config, false)

							if err != nil {
								if err == filepath.SkipDir {
									debugLog(&config, "Directory skipped: %s", event.Name)
								} else {
									debugLog(&config, "Error watching new directory: %v", err)
								}
							}

							continue
						}

						// Skip hidden and special files
						if IsHiddenOrSpecialFile(event.Name) {
							debugLog(&config, "Skipping hidden or special file: %s", event.Name)
							continue
						}

						// Check if file should be ignored based on patterns
						if shouldIgnore, reason := ShouldIgnorePathWithConfig(event.Name, &config); shouldIgnore {
							debugLog(&config, "Skipping file due to %s: %s", reason, event.Name)
							continue
						}
						debugLog(&config, "Watching file: %s", event.Name)

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

						markers := findActiveAIMarkers(string(content))
						if len(markers) > 0 {
							absPath, err := filepath.Abs(event.Name)
							if err != nil {
								continue
							}

							// Store original markers for logging
							originalMarkers := make([]AIMarkerLocation, len(markers))
							copy(originalMarkers, markers)

							// Log file change before processing
							errorLog(&config, "File change detected: %s - sending to Claude", event.Name)
							for _, marker := range originalMarkers {
								errorLog(&config, "  Line %d: %s", marker.LineNumber, marker.LineText)
							}

							// Remove AI markers from the file and get updated markers
							debugLog(&config, "Removing AI markers from file: %s", event.Name)
							updatedMarkers, err := removeAIMarkersFromFile(event.Name, markers)
							if err != nil {
								errorLog(&config, "Error removing AI markers: %v", err)
								continue
							}
							debugLog(&config, "AI markers successfully removed from file")

							// Log the updated markers for debugging
							if config.Debug {
								for i, marker := range updatedMarkers {
									debugLog(&config, "  Original: Line %d: %s", originalMarkers[i].LineNumber, originalMarkers[i].LineText)
									debugLog(&config, "  Updated:  Line %d: %s", marker.LineNumber, marker.LineText)
								}
							}

							// Prepare the template data with the updated markers
							data := TemplateData{
								File:    absPath,
								Markers: updatedMarkers,
							}

							// Execute the template (resolved per file, cached per dir)
							promptTmpl := resolver.resolve(absPath)
							var promptBuf strings.Builder
							err = promptTmpl.Execute(&promptBuf, data)
							if err != nil {
								errorLog(&config, "Error executing prompt template: %v", err)
								continue
							}

							// Send the generated prompt to the channel for processing
							promptChan <- promptBuf.String()
						}
					}

				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					errorLog(&config, "Error: %v", err)
				}
			}
		}()

		// Process prompts from file changes, injecting each one via
		// bracketed paste when Claude's TUI supports it (see injector.go)
		// so multi-line prompts land as one literal block, and stopping
		// cleanly once Claude has exited.
		inj := &injector{
			writer:     syncedPty,
			detector:   pasteDetector,
			claudeDone: claudeDone,
			errorLogFn: func(format string, args ...interface{}) { errorLog(&config, format, args...) },
			debugLogFn: func(format string, args ...interface{}) { debugLog(&config, format, args...) },
		}
		for prompt := range promptChan {
			if err := inj.inject(prompt); err != nil && err != errClaudeExited {
				errorLog(&config, "Error injecting prompt: %v", err)
			}
		}
	}()

	// Wait for Claude to finish
	<-claudeDone
	if claudeWaitErr != nil {
		fmt.Fprintf(os.Stderr, "Claude process ended with error: %v\n", claudeWaitErr)
	}

	// Close the prompt channel and wait for goroutines to finish
	close(promptChan)
	wg.Wait()
}
