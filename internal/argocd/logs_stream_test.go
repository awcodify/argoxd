package argocd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
)

var webPod = explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: "store", Name: "web-1-abc"}

func receive(t *testing.T, stream <-chan LogEntry) LogEntry {
	t.Helper()
	select {
	case entry, open := <-stream:
		if !open {
			t.Fatal("the stream closed early")
		}
		return entry
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a log entry")
	}
	return LogEntry{}
}

func expectClosed(t *testing.T, stream <-chan LogEntry) {
	t.Helper()
	select {
	case entry, open := <-stream:
		if open {
			t.Fatalf("the stream sent %+v instead of closing", entry)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the stream did not close")
	}
}

func logEvent(line string) string {
	return `{"result":{"content":"` + line + `"}}` + "\n"
}

func TestAPISourceStreamsLogsUntilCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if query := r.URL.Query(); r.URL.Path != "/api/v1/applications/checkout/logs" || query.Get("follow") != "true" || query.Get("podName") != "web-1-abc" {
			t.Errorf("request = %s?%v", r.URL.Path, query)
		}
		_, _ = w.Write([]byte(logEvent("one") + logEvent("two")))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := NewAPISource(server.URL, "", false).StreamLogs(ctx, "checkout", webPod)
	if err != nil {
		t.Fatal(err)
	}

	if first, second := receive(t, stream), receive(t, stream); first.Line != "one" || second.Line != "two" {
		t.Fatalf("entries = %+v, %+v", first, second)
	}
	cancel()
	expectClosed(t, stream)
}

func TestAPISourceStreamIsNotCutOffByTheRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(logEvent("early")))
		w.(http.Flusher).Flush()
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(logEvent("late")))
	}))
	defer server.Close()
	source := NewAPISource(server.URL, "", false)
	source.client.Timeout = 50 * time.Millisecond

	stream, err := source.StreamLogs(context.Background(), "checkout", webPod)
	if err != nil {
		t.Fatal(err)
	}

	if first, second := receive(t, stream), receive(t, stream); first.Line != "early" || second.Line != "late" {
		t.Fatalf("entries = %+v, %+v", first, second)
	}
	expectClosed(t, stream)
}

func TestAPISourceStreamReportsRefusals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"permission denied"}`))
	}))
	defer server.Close()

	_, err := NewAPISource(server.URL, "", false).StreamLogs(context.Background(), "checkout", webPod)

	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("error = %v, want the refusal with Argo CD's message", err)
	}
}

func TestAPISourceStreamReportsAConnectionThatDrops(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(logEvent("before")))
		w.(http.Flusher).Flush()
		connection, _, _ := w.(http.Hijacker).Hijack()
		_ = connection.Close()
	}))
	defer server.Close()

	stream, err := NewAPISource(server.URL, "", false).StreamLogs(context.Background(), "checkout", webPod)
	if err != nil {
		t.Fatal(err)
	}

	if entry := receive(t, stream); entry.Line != "before" {
		t.Fatalf("first entry = %+v", entry)
	}
	if entry := receive(t, stream); entry.Err == nil {
		t.Fatalf("entry = %+v, want the error that ended the stream", entry)
	}
	expectClosed(t, stream)
}

func TestKubernetesSourceStreamsLogs(t *testing.T) {
	source := fakeKubernetesSource()

	stream, err := source.StreamLogs(context.Background(), "checkout", webPod)
	if err != nil {
		t.Fatal(err)
	}

	if entry := receive(t, stream); entry.Line != "fake logs" || entry.Err != nil {
		t.Fatalf("entry = %+v, want the pod's log line", entry)
	}
	expectClosed(t, stream)
}

func TestDemoSourceStreamsNewLinesUntilCancelled(t *testing.T) {
	source := NewDemoSource()
	source.logInterval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pod := explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: "store", Name: "cart-5d8f7c-x7k2p", Health: "Healthy"}

	stream, err := source.StreamLogs(ctx, "cart", pod)
	if err != nil {
		t.Fatal(err)
	}

	if first := receive(t, stream); !strings.Contains(first.Line, "starting cart-5d8f7c-x7k2p") {
		t.Fatalf("first line = %q, want the recent log to come first", first.Line)
	}
	for range 8 {
		if entry := receive(t, stream); entry.Err != nil || entry.Line == "" {
			t.Fatalf("entry = %+v", entry)
		}
	}
	cancel()
	for range 100 {
		if _, open := <-stream; !open {
			return
		}
	}
	t.Fatal("the stream did not close after cancel")
}

func TestDemoSourceStreamRefusesUnknownApplications(t *testing.T) {
	if _, err := NewDemoSource().StreamLogs(context.Background(), "missing", webPod); err == nil {
		t.Fatal("streaming a missing application succeeded")
	}
}
