package utils

import (
	"io"
	"sync"
)

type PrefixWriter struct {
	w           io.Writer
	prefix      string
	mu          sync.Mutex
	atLineStart bool
}

func NewPrefixWriter(w io.Writer, prefix string) *PrefixWriter {
	return &PrefixWriter{
		w:           w,
		prefix:      prefix,
		atLineStart: true,
	}
}

func (pw *PrefixWriter) Write(p []byte) (int, error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	total := 0
	for len(p) > 0 {
		if pw.atLineStart {
			_, err := pw.w.Write([]byte(pw.prefix))
			if err != nil {
				return total, err
			}
			pw.atLineStart = false
		}
		i := 0
		for i < len(p) && p[i] != '\n' {
			i++
		}
		if i < len(p) { // found newline
			n, err := pw.w.Write(p[:i+1])
			total += n
			if err != nil {
				return total, err
			}
			pw.atLineStart = true
			p = p[i+1:]
		} else {
			n, err := pw.w.Write(p)
			total += n
			if err != nil {
				return total, err
			}
			break
		}
	}
	return total, nil
}

func (pw *PrefixWriter) EnsureLineEnd() error {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	if pw.atLineStart {
		return nil
	}
	_, err := pw.w.Write([]byte("\n"))
	if err != nil {
		return err
	}
	pw.atLineStart = true
	return nil
}
