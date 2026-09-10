package api

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
)

func TestEventsSurviveOrdinaryWriteDeadlineAndReleaseOnDisconnect(t *testing.T) {
	finished := make(chan struct{})
	events := make(chan scan.Event, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		w.Header().Set("Content-Type", "text/event-stream")
		serveEvents(w, r, events, 20*time.Millisecond)
	}))
	// Scale the ordinary 30-second timeout down so this regression remains fast.
	server.Config.WriteTimeout = 50 * time.Millisecond
	server.Start()
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	started := time.Now()
	heartbeats := 0
	for time.Since(started) < 150*time.Millisecond {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("event stream ended at %v: %v", time.Since(started), err)
		}
		if strings.HasPrefix(line, ": keepalive") {
			heartbeats++
		}
	}
	if heartbeats < 2 {
		t.Fatalf("missing heartbeats: %d", heartbeats)
	}
	events <- scan.Event{Kind: "complete", At: time.Now().UTC()}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "event: complete") {
			break
		}
	}
	response.Body.Close()
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("disconnected stream retained its handler/subscription")
	}
}

type failingStream struct {
	header          http.Header
	failure         string
	writes, flushes int
	deadline        time.Time
}

func (w *failingStream) Header() http.Header { return w.header }
func (w *failingStream) WriteHeader(int)     {}
func (w *failingStream) Write(p []byte) (int, error) {
	w.writes++
	if w.failure == "write" {
		return 0, errors.New("disconnected")
	}
	return len(p), nil
}
func (w *failingStream) FlushError() error {
	w.flushes++
	if w.failure == "flush" {
		return errors.New("disconnected")
	}
	return nil
}
func (w *failingStream) SetWriteDeadline(at time.Time) error {
	w.deadline = at
	if w.failure == "deadline" {
		return errors.New("unsupported")
	}
	return nil
}

func TestEventsExitOnWriteErrorsOrClosedSource(t *testing.T) {
	for _, failure := range []string{"deadline", "write", "flush", ""} {
		t.Run(failure, func(t *testing.T) {
			writer := &failingStream{header: make(http.Header), failure: failure}
			events := make(chan scan.Event)
			close(events)
			serveEvents(writer, httptest.NewRequest(http.MethodGet, "/", nil), events, time.Hour)
			if writer.writes > 1 || writer.flushes > 1 {
				t.Fatal("stream continued after a failed write or closed channel")
			}
			if failure == "" && !writer.deadline.IsZero() {
				t.Fatal("idle stream retained a total write deadline")
			}
		})
	}
}
