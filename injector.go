package main

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// bracketedPasteEnableSeq is the escape sequence a terminal application sends
// when it wants pasted text reported as a single literal block instead of as
// individual keystrokes. If we observe Claude's TUI emit it, we know it's
// safe to wrap injected prompts the same way a real paste would be reported,
// so the TUI treats a multi-line prompt as one atomic block rather than
// risking embedded newlines being read as separate Enter presses.
const bracketedPasteEnableSeq = "\x1b[?2004h"

const (
	bracketedPasteStart = "\x1b[200~"
	bracketedPasteEnd   = "\x1b[201~"

	// submitDelay separates the pasted content from the submit keystroke.
	// This is needed even with bracketed paste: two back-to-back writes can
	// still be coalesced into a single read on Claude's side, so a bare CR
	// immediately following the paste-end marker was observed (manual
	// testing) to insert the text without submitting it. A real pause
	// forces the submit keystroke to arrive as its own read/event.
	submitDelay = 300 * time.Millisecond
)

// errClaudeExited is returned by injector.inject when the wrapped Claude
// process has already exited, so callers can distinguish "nothing to do"
// from a genuine write failure.
var errClaudeExited = errors.New("claude process exited")

// syncWriter serializes writes to an underlying io.Writer. claudewatch has
// two independent sources of input into Claude's PTY - the human's forwarded
// keystrokes and injected marker-fix prompts - and without this, concurrent
// writes from both could interleave at the byte level.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// pasteModeDetector watches Claude's output stream for the bracketed-paste
// enable sequence. It's fed chunks as they're copied through to the real
// terminal, and keeps a small carry-over buffer so the sequence is still
// recognized if it happens to land across two reads.
type pasteModeDetector struct {
	mu      sync.Mutex
	enabled bool
	carry   []byte
}

func (d *pasteModeDetector) feed(chunk []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.enabled {
		return
	}

	buf := append(d.carry, chunk...)
	if containsBytes(buf, bracketedPasteEnableSeq) {
		d.enabled = true
		d.carry = nil
		return
	}

	// Keep only enough of a suffix to catch a sequence split across chunks.
	keep := len(bracketedPasteEnableSeq) - 1
	if len(buf) > keep {
		buf = buf[len(buf)-keep:]
	}
	d.carry = append([]byte(nil), buf...)
}

func (d *pasteModeDetector) isEnabled() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.enabled
}

func containsBytes(haystack []byte, needle string) bool {
	n := len(needle)
	if n == 0 || len(haystack) < n {
		return false
	}
	for i := 0; i+n <= len(haystack); i++ {
		if string(haystack[i:i+n]) == needle {
			return true
		}
	}
	return false
}

// copyAndDetectPaste copies src to dst, exactly like io.Copy, while also
// feeding every chunk read to detector. It exists so we can observe Claude's
// output stream without adding any latency to what the human sees.
func copyAndDetectPaste(dst io.Writer, src io.Reader, detector *pasteModeDetector) error {
	buf := make([]byte, 4096)
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			detector.feed(buf[:n])
			if _, err := dst.Write(buf[:n]); err != nil {
				return err
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return readErr
		}
	}
}

// injector delivers a marker-fix prompt into Claude's PTY.
type injector struct {
	writer     io.Writer
	detector   *pasteModeDetector
	claudeDone <-chan struct{}
	errorLogFn func(format string, args ...interface{})
	debugLogFn func(format string, args ...interface{})
}

// inject writes prompt to Claude's PTY and submits it. When bracketed paste
// has been detected, the prompt is wrapped so a multi-line prompt lands as
// one literal block rather than risking embedded newlines being read as
// separate Enter presses. Either way, the submit keystroke (CR) is always
// sent as its own write after a real pause - see the submitDelay comment
// for why that pause can't be dropped even with bracketed paste.
func (inj *injector) inject(prompt string) error {
	select {
	case <-inj.claudeDone:
		inj.logError("Claude process has exited; dropping queued prompt")
		return errClaudeExited
	default:
	}

	content := prompt
	if inj.detector.isEnabled() {
		inj.logDebug("Injecting prompt using bracketed paste")
		content = bracketedPasteStart + prompt + bracketedPasteEnd
	} else {
		inj.logDebug("Bracketed paste not detected; using raw injection")
	}

	if _, err := inj.writer.Write([]byte(content)); err != nil {
		return fmt.Errorf("writing prompt: %w", err)
	}

	time.Sleep(submitDelay)

	if _, err := inj.writer.Write([]byte{13}); err != nil {
		return fmt.Errorf("writing submit: %w", err)
	}
	return nil
}

func (inj *injector) logError(format string, args ...interface{}) {
	if inj.errorLogFn != nil {
		inj.errorLogFn(format, args...)
	}
}

func (inj *injector) logDebug(format string, args ...interface{}) {
	if inj.debugLogFn != nil {
		inj.debugLogFn(format, args...)
	}
}
