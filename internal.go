package voicy

import (
	"io"
	"time"
)

type internalWriter struct{ *Session }

func (iw internalWriter) Write(b []byte) (n int, err error) {
	if iw.state == PausedState {
		iw.waitAnyState()
	}

	if iw.state != PlayingState {
		return 0, io.EOF
	}

	n, err = iw.conn.Write(b)
	if err == nil {
		iw.position += (20 * time.Millisecond)
	}

	return
}
