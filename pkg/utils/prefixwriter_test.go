package utils

import (
	"bytes"
	"testing"
)

func TestPrefixWriterEnsureLineEndAddsNewlineForPartialLine(t *testing.T) {
	var out bytes.Buffer
	writer := NewPrefixWriter(&out, "[runner > component] ")

	if _, err := writer.Write([]byte("partial")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := writer.EnsureLineEnd(); err != nil {
		t.Fatalf("ensure line end failed: %v", err)
	}

	want := "[runner > component] partial\n"
	if got := out.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPrefixWriterEnsureLineEndDoesNotAddExtraNewline(t *testing.T) {
	var out bytes.Buffer
	writer := NewPrefixWriter(&out, "[runner > component] ")

	if _, err := writer.Write([]byte("complete\n")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := writer.EnsureLineEnd(); err != nil {
		t.Fatalf("ensure line end failed: %v", err)
	}

	want := "[runner > component] complete\n"
	if got := out.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
