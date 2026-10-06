package argocd

import (
	"bufio"
	"context"
	"io"
	"sync"
)

// maxLogLineBytes is the longest log line a followed stream reads.
const maxLogLineBytes = 1 << 20

// maxFollowedPods bounds how many Pods of one workload are followed at once.
const maxFollowedPods = 30

// streamLines sends the lines of a log stream, as read by parse, until the
// body ends, fails or ctx is cancelled. A failure is sent as a last entry
// with Err set. The body is closed and the channel is closed when it is done.
func streamLines(ctx context.Context, body io.ReadCloser, parse func([]byte) (LogEntry, bool)) <-chan LogEntry {
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
			if entry, ok := parse(scanner.Bytes()); ok && !send(entry) {
				return
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			send(LogEntry{Err: err})
		}
	}()
	return entries
}

// mergeStreams sends the entries of every stream on one channel, which closes
// when all of them have ended or ctx is cancelled. One Pod failing must not
// end the logs of the others, so a failure is passed on as a line of its Pod.
func mergeStreams(ctx context.Context, streams ...<-chan LogEntry) <-chan LogEntry {
	merged := make(chan LogEntry)
	var running sync.WaitGroup
	for _, stream := range streams {
		running.Add(1)
		go func() {
			defer running.Done()
			for {
				select {
				case entry, open := <-stream:
					if !open {
						return
					}
					if entry.Err != nil {
						entry = LogEntry{Pod: entry.Pod, Line: "stream error: " + entry.Err.Error()}
					}
					select {
					case merged <- entry:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		running.Wait()
		close(merged)
	}()
	return merged
}

// singleEntry is a stream of one entry, for a notice among the lines of Pods.
func singleEntry(entry LogEntry) <-chan LogEntry {
	stream := make(chan LogEntry, 1)
	stream <- entry
	close(stream)
	return stream
}
