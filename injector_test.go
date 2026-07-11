package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

func TestPasteModeDetector_WholeSequenceInOneChunk(t *testing.T) {
	d := &pasteModeDetector{}
	d.feed([]byte("hello " + bracketedPasteEnableSeq + " world"))
	if !d.isEnabled() {
		t.Fatal("expected detector to be enabled after seeing the full sequence")
	}
}

func TestPasteModeDetector_SequenceSplitAcrossChunks(t *testing.T) {
	seq := bracketedPasteEnableSeq
	split := len(seq) / 2

	d := &pasteModeDetector{}
	d.feed([]byte("preamble " + seq[:split]))
	if d.isEnabled() {
		t.Fatal("should not be enabled from a partial sequence")
	}
	d.feed([]byte(seq[split:] + " trailer"))
	if !d.isEnabled() {
		t.Fatal("expected detector to be enabled once the split sequence completes")
	}
}

func TestPasteModeDetector_NoFalsePositive(t *testing.T) {
	d := &pasteModeDetector{}
	d.feed([]byte("just some regular Claude TUI output with no escape codes"))
	if d.isEnabled() {
		t.Fatal("should not be enabled without the enable sequence")
	}
}

func TestPasteModeDetector_StaysEnabledOnceSet(t *testing.T) {
	d := &pasteModeDetector{}
	d.feed([]byte(bracketedPasteEnableSeq))
	d.feed([]byte("more output that doesn't matter now"))
	if !d.isEnabled() {
		t.Fatal("expected detector to remain enabled")
	}
}

func TestCopyAndDetectPaste_CopiesAndFeedsDetector(t *testing.T) {
	src := strings.NewReader("some output " + bracketedPasteEnableSeq + " more")
	var dst bytes.Buffer
	detector := &pasteModeDetector{}

	if err := copyAndDetectPaste(&dst, src, detector); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dst.String() != "some output "+bracketedPasteEnableSeq+" more" {
		t.Fatalf("copy did not preserve bytes: %q", dst.String())
	}
	if !detector.isEnabled() {
		t.Fatal("expected detector to observe the enable sequence during copy")
	}
}

type erroringReader struct{ err error }

func (r erroringReader) Read([]byte) (int, error) { return 0, r.err }

func TestCopyAndDetectPaste_PropagatesReadError(t *testing.T) {
	boom := errors.New("boom")
	var dst bytes.Buffer
	err := copyAndDetectPaste(&dst, erroringReader{err: boom}, &pasteModeDetector{})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom error, got %v", err)
	}
}

func TestInjector_BracketedPasteSendsWrappedContentThenSeparateCR(t *testing.T) {
	var writes [][]byte
	rec := recordingWriter(func(p []byte) { writes = append(writes, append([]byte(nil), p...)) })

	detector := &pasteModeDetector{}
	detector.feed([]byte(bracketedPasteEnableSeq))

	inj := &injector{
		writer:     rec,
		detector:   detector,
		claudeDone: make(chan struct{}),
	}

	if err := inj.inject("do the thing"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The submit keystroke must be its own write, sent after the delay -
	// concatenating it onto the paste-end marker (or writing it back-to-back
	// with no pause) was observed, via manual testing, to insert the text
	// without submitting it.
	if len(writes) != 2 {
		t.Fatalf("expected two writes (content, then CR), got %d", len(writes))
	}
	wantContent := bracketedPasteStart + "do the thing" + bracketedPasteEnd
	if string(writes[0]) != wantContent {
		t.Fatalf("unexpected content write: %q", string(writes[0]))
	}
	if len(writes[1]) != 1 || writes[1][0] != 13 {
		t.Fatalf("expected second write to be a lone CR byte, got %v", writes[1])
	}
}

func TestInjector_FallbackSendsPromptThenCR(t *testing.T) {
	var writes [][]byte
	rec := recordingWriter(func(p []byte) { writes = append(writes, append([]byte(nil), p...)) })

	inj := &injector{
		writer:     rec,
		detector:   &pasteModeDetector{}, // not enabled
		claudeDone: make(chan struct{}),
	}

	if err := inj.inject("fix it"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(writes) != 2 {
		t.Fatalf("expected two writes in fallback mode, got %d", len(writes))
	}
	if string(writes[0]) != "fix it" {
		t.Fatalf("unexpected first write: %q", string(writes[0]))
	}
	if len(writes[1]) != 1 || writes[1][0] != 13 {
		t.Fatalf("expected second write to be a lone CR byte, got %v", writes[1])
	}
}

func TestInjector_SkipsWriteWhenClaudeExited(t *testing.T) {
	done := make(chan struct{})
	close(done)

	writeCalled := false
	rec := recordingWriter(func(p []byte) { writeCalled = true })

	inj := &injector{
		writer:     rec,
		detector:   &pasteModeDetector{},
		claudeDone: done,
	}

	err := inj.inject("too late")
	if !errors.Is(err, errClaudeExited) {
		t.Fatalf("expected errClaudeExited, got %v", err)
	}
	if writeCalled {
		t.Fatal("should not write to the PTY once Claude has exited")
	}
}

// recordingWriter is a minimal io.Writer for asserting on write calls.
type recordingWriter func([]byte)

func (r recordingWriter) Write(p []byte) (int, error) {
	r(p)
	return len(p), nil
}

func TestSyncWriter_SerializesConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	sw := &syncWriter{w: &buf}

	const goroutines = 20
	const perGoroutine = 50
	payload := []byte("x")

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				if _, err := sw.Write(payload); err != nil {
					t.Errorf("unexpected write error: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	if got, want := buf.Len(), goroutines*perGoroutine; got != want {
		t.Fatalf("expected %d bytes written, got %d (interleaving/loss under concurrency)", want, got)
	}
}

var _ io.Writer = (*syncWriter)(nil)
