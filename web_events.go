package main

import (
	"fmt"
	"net/http"
	"sync"
)

type eventBus struct {
	mu    sync.RWMutex
	chans map[chan string]struct{}
}

var bus = &eventBus{chans: map[chan string]struct{}{}}

func (b *eventBus) subscribe() chan string {
	ch := make(chan string, 32)
	b.mu.Lock()
	b.chans[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *eventBus) unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.chans, ch)
	b.mu.Unlock()
}

func (b *eventBus) publish(eventType, data string) {
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.chans {
		select {
		case ch <- msg:
		default:
		}
	}
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "SSE not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := bus.subscribe()
	defer bus.unsubscribe(ch)

	fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
	fmt.Fprintf(w, "event: engine\ndata: %s\n\n", mustJSON(engine.GetState()))
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			w.Write([]byte(msg))
			flusher.Flush()
		}
	}
}
