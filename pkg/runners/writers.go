package runners

import "io"

type lineEndEnsurer interface {
	EnsureLineEnd() error
}

func ensureExecWriterLineEnds(writers ...io.Writer) {
	for _, writer := range writers {
		ensurer, ok := writer.(lineEndEnsurer)
		if !ok {
			continue
		}
		_ = ensurer.EnsureLineEnd()
	}
}
