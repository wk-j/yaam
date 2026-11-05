package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// hub fans server-sent events out to every open /api/events stream.
type hub struct {
	mu   sync.Mutex
	subs map[chan []byte]bool
}

func newHub() *hub { return &hub{subs: map[chan []byte]bool{}} }

func sseMessage(event string, v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err) // only yaam's own types are sent
	}
	return fmt.Appendf(nil, "event: %s\ndata: %s\n\n", event, data)
}

func (h *hub) subscribe() (chan []byte, func()) {
	ch := make(chan []byte, 256)
	h.mu.Lock()
	h.subs[ch] = true
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.subs[ch] {
			delete(h.subs, ch)
			close(ch)
		}
	}
}

// publish never blocks: a stream too slow to keep up is closed, and the page
// reconnects and starts again from a full state event.
func (h *hub) publish(event string, v any) {
	msg := sseMessage(event, v)
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- msg:
		default:
			delete(h.subs, ch)
			close(ch)
		}
	}
}

const pingInterval = 20 * time.Second

func (a *App) events(w http.ResponseWriter, r *http.Request) {
	ch, unsubscribe := a.hub.subscribe()
	defer unsubscribe()
	w.Header().Set("Content-Type", "text/event-stream")
	rc := http.NewResponseController(w)
	if _, err := w.Write(sseMessage("state", a.state())); err != nil {
		return
	}
	rc.Flush()
	ping := time.NewTicker(pingInterval)
	defer ping.Stop()
	for {
		var msg []byte
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			msg = []byte(": ping\n\n")
		case m, ok := <-ch:
			if !ok {
				return
			}
			msg = m
		}
		if _, err := w.Write(msg); err != nil {
			return
		}
		if rc.Flush() != nil {
			return
		}
	}
}
