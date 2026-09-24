package api

import (
	"encoding/json"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
	"io"
	"net/http"
	"time"
)

func (s *Server) events(writer http.ResponseWriter, request *http.Request, id string) {
	if _, err := s.manager.Get(request.Context(), id); err != nil {
		writeError(writer, http.StatusNotFound, "scan not found")
		return
	}
	_, supported := writer.(http.Flusher)
	if !supported {
		writeError(writer, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	events, unsubscribe := s.manager.Subscribe(id)
	defer unsubscribe()
	serveEvents(writer, request, events, 15*time.Second)
}

// SSE has no total response deadline. Bound each write instead, so a slow
// reader cannot retain a subscription and the server's ordinary WriteTimeout
// does not disconnect healthy long-running scans.
func serveEvents(writer http.ResponseWriter, request *http.Request, events <-chan scan.Event, heartbeat time.Duration) {
	control := http.NewResponseController(writer)
	write := func(payload string) error {
		if err := control.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return err
		}
		if _, err := io.WriteString(writer, payload); err != nil {
			return err
		}
		if err := control.Flush(); err != nil {
			return err
		}
		return control.SetWriteDeadline(time.Time{})
	}
	writeEvent := func(event scan.Event) error {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		return write(fmt.Sprintf("event: %s\ndata: %s\n\n", event.Kind, payload))
	}
	if err := writeEvent(scan.Event{Kind: "connected", Message: "event stream connected", At: time.Now().UTC()}); err != nil {
		return
	}
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case event, open := <-events:
			if !open || writeEvent(event) != nil {
				return
			}
		case <-ticker.C:
			if err := write(": keepalive\n\n"); err != nil {
				return
			}
		}
	}
}
