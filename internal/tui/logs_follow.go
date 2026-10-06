package tui

import (
	"context"
	"errors"

	"github.com/awcodify/argoxd/internal/argocd"
	tea "github.com/charmbracelet/bubbletea"
)

// maxLogLines is how many lines a followed log keeps; older ones are dropped.
const maxLogLines = 10000

var errNoStreaming = errors.New("the active source cannot follow logs")

// logStreamStarted reports that the stream with the given id is open, or why it is not.
type logStreamStarted struct {
	stream  int
	entries <-chan argocd.LogEntry
	err     error
}

// logLine is an entry of the stream with the given id.
type logLine struct {
	stream int
	entry  argocd.LogEntry
}

// logStreamEnded reports that the stream with the given id was closed.
type logStreamEnded struct {
	stream int
}

// followState is the log stream being followed. The id tells its messages
// apart from those of an earlier stream.
type followState struct {
	id      int
	entries <-chan argocd.LogEntry
	cancel  context.CancelFunc
}

func (m Model) following() bool {
	return m.follow.cancel != nil
}

// toggleFollow starts following the open Pod log, or stops following it.
func (m Model) toggleFollow() (tea.Model, tea.Cmd) {
	if m.viewer.kind != "logs" {
		return m, nil
	}
	if m.following() {
		m.stopFollowing()
		return m, nil
	}
	streamer, ok := m.source.(argocd.LogStreamer)
	if !ok {
		m.err = errNoStreaming
		return m, nil
	}

	// A stream has no timeout: it stays open until it is stopped or ends.
	ctx, cancel := context.WithCancel(context.Background())
	id := m.follow.id + 1
	m.follow = followState{id: id, cancel: cancel}
	m.err, m.status = nil, ""
	// The stream starts with the recent lines, so it replaces the snapshot.
	m.viewer.lines, m.viewer.offset = nil, 0
	application, pod := m.resourceTree.Application, m.logPod
	return m, func() tea.Msg {
		entries, err := streamer.StreamLogs(ctx, application, pod)
		return logStreamStarted{stream: id, entries: entries, err: err}
	}
}

// stopFollowing cancels the stream. Its late messages are ignored.
func (m *Model) stopFollowing() {
	if m.follow.cancel != nil {
		m.follow.cancel()
	}
	m.follow = followState{id: m.follow.id}
}

func (m Model) applyLogStream(message logStreamStarted) (tea.Model, tea.Cmd) {
	if !m.following() || message.stream != m.follow.id {
		return m, nil
	}
	if message.err != nil {
		m.err = message.err
		m.stopFollowing()
		return m, nil
	}
	m.follow.entries = message.entries
	return m, m.nextLogLine()
}

func (m Model) applyLogLine(message logLine) (tea.Model, tea.Cmd) {
	if !m.following() || message.stream != m.follow.id {
		return m, nil
	}
	if message.entry.Err != nil {
		m.err = message.entry.Err
		m.stopFollowing()
		return m, nil
	}
	m.appendLogLine(message.entry.Line)
	return m, m.nextLogLine()
}

// nextLogLine waits for the next entry of the stream being followed.
func (m Model) nextLogLine() tea.Cmd {
	id, entries := m.follow.id, m.follow.entries
	return func() tea.Msg {
		entry, open := <-entries
		if !open {
			return logStreamEnded{stream: id}
		}
		return logLine{stream: id, entry: entry}
	}
}

// appendLogLine adds a line to the viewer, keeping the view at the bottom if it
// was there and dropping the oldest lines beyond maxLogLines.
func (m *Model) appendLogLine(line string) {
	pinned := m.viewer.offset >= max(0, len(m.viewer.lines)-m.bodyHeight())
	m.viewer.lines = append(m.viewer.lines, line)
	if dropped := len(m.viewer.lines) - maxLogLines; dropped > 0 {
		m.viewer.lines = m.viewer.lines[dropped:]
		m.viewer.offset = max(0, m.viewer.offset-dropped)
	}
	if pinned {
		m.viewer.offset = max(0, len(m.viewer.lines)-m.bodyHeight())
	}
}
