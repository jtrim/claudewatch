# claudewatch

A transparent wrapper for the Claude CLI that watches file changes and automatically sends AI-directed instructions to Claude.

## Overview

`claudewatch` acts as a transparent passthrough for the Claude CLI, with added file watching functionality. When files
with special AI-directed comments are modified, it automatically sends the file to Claude with instructions to modify it
according to the comments.

## Features

- Acts as a transparent wrapper for the Claude CLI - use it exactly as you would use the Claude CLI
- Watches for file changes in the specified directory
- Detects comments ending with "ai!" in changed files
- Automatically sends files with AI comments to Claude with specific instructions
- Customizable prompt template with `--prompt` flag
- Debug mode with `--debug` flag
- Support for `.claudewatchignore` file with regex patterns to exclude files from watching

## Installation

Run:

```
$ go install github.com/jtrim/claudewatch@latest
```

## Requirements

- Claude CLI installed and available in your PATH
- For development: Go 1.18 or later

## Usage

### Basic Usage

```bash
./claudewatch [options] [directory] [-- claude_arguments]
```

By default, `claudewatch` watches the current directory. You can specify a different directory to watch as an argument. Use the `--` separator to pass arguments directly to the Claude CLI.

### Command Line Arguments

- `--debug`: Enable debug output
- `--prompt "template text"`: Customize the prompt template (use `{{.File}}` as a variable for the file path)
- `--`: Everything after this marker is passed directly to Claude

### Examples

```bash
# Basic usage - watches current directory
./claudewatch

# Watch a specific directory
./claudewatch /path/to/project

# Enable debug output
./claudewatch --debug

# Use a custom prompt template
./claudewatch --prompt "Please modify {{.File}} according to the 'ai!' comments."

# Pass arguments to Claude
./claudewatch -- --model-name claude-3-opus-20240229

# Combined usage
./claudewatch --debug /path/to/project -- --model-name claude-3-opus-20240229
```

## How It Works

1. `claudewatch` starts Claude CLI with a pseudo-terminal (PTY)
2. It watches the specified directory for file changes
3. When a file changes, it checks for comments ending with "ai!"
4. If such comments are found, it sends a prompt to Claude with the file path
5. Claude processes the prompt and modifies the file as instructed

## AI Comment Format

Any comment ending with one of the supported markers (`ai!`, `!ai`, or `ai?`) will be detected. Markers are case-insensitive:

```go
// Change this function to use a Map instead of a Slice ai!
```

```python
# Fix the bug in this function !ai
```

```js
/* Refactor this code to be more efficient AI? */
```

### Ignoring AI Instructions

You can use `ai:ignore` (also case-insensitive) to prevent processing of an AI instruction:

```go
// ai:ignore
// This comment with an AI marker won't be processed ai!
```

You can also put both on the same line:

```python
# ai:ignore ai! This instruction will be ignored
```

### Ignoring Files with .claudewatchignore

You can create a `.claudewatchignore` file in the root directory being watched to exclude files from being processed. The file should contain one Go-style regular expression pattern per line:

```
# This is a comment in the .claudewatchignore file
\.js$          # Ignore all JavaScript files
node_modules/  # Ignore files in the node_modules directory
test_.*\.go    # Ignore Go test files with names starting with test_
```

Blank lines and lines starting with `#` are ignored. Each regex pattern is applied to the full file path, and if there's a match, the file is excluded from being processed.

## Disclaimer

⚠️ **EXPERIMENTAL SOFTWARE**: `claudewatch` is experimental software provided "as is" without any warranties or guarantees of any kind, either expressed or implied. By using this software, you acknowledge and accept that:

- It may contain bugs, errors, or security vulnerabilities
- It may not function as expected or may fail entirely
- It may be modified or discontinued at any time without notice
- No guarantees of performance, reliability, or suitability for any purpose are made

**USE AT YOUR OWN RISK**.

## License

MIT
