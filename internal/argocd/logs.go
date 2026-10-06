package argocd

import (
	"bufio"
	"context"
	"io"
)

// maxLogLineBytes is the longest log line a followed stream reads.
const maxLogLineBytes = 1 << 20

// streamLines sends the lines of a log stream, as read by parse, until the
// body ends, fails or ctx is cancelled. A failure is sent as a last entry
// with Err set. The body is closed and the channel is closed when it is done.
func streamLines(ctx context.Context, body io.ReadCloser, parse func([]byte) (string, bool)) <-chan LogEntry {
	entries := make(chan LogEntry)
	go func() {
		defer close(entries)
		defer body.Close()
		// Cancelling ctx closes the body, which unblocks a read that is waiting.
		defer context.AfterFunc(ctx, func() { _ = body.Close() })()

		send := func(entry LogEntry) bool {
			select {
			case entries <- entry:
				return true
			case <-ctx.Done():
				return false
			}
		}
		scanner := bufio.NewScanner(body)
		scanner.Buffer(nil, maxLogLineBytes)
		for scanner.Scan() {
			if line, ok := parse(scanner.Bytes()); ok && !send(LogEntry{Line: line}) {
				return
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			send(LogEntry{Err: err})
		}
	}()
	return entries
}
