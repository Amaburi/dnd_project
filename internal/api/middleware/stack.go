package middleware

import "runtime"

// stack captures the goroutine's stack for a recovered panic.
//
// Bounded rather than unbounded: a runaway recursion would otherwise write
// megabytes into a log line.
func stack() []byte {
	buf := make([]byte, 8192)
	n := runtime.Stack(buf, false)
	return buf[:n]
}
