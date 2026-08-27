package service

import "io"

// discard is an io.Writer that drops all output, for quiet tests.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

var _ io.Writer = discard{}
