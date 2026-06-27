package main

import "sync"

type eventHub struct {
	mu      sync.RWMutex
	closed  bool
	clients map[chan sseEvent]struct{}
}

func newEventHub() *eventHub {
	return &eventHub{clients: map[chan sseEvent]struct{}{}}
}

func (h *eventHub) subscribe() chan sseEvent {
	ch := make(chan sseEvent, clientEventQueueSize)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(ch)
		return ch
	}
	h.clients[ch] = struct{}{}
	return ch
}

func (h *eventHub) unsubscribe(ch chan sseEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
}

func (h *eventHub) publish(event sseEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.closed {
		return
	}
	for ch := range h.clients {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *eventHub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for ch := range h.clients {
		close(ch)
		delete(h.clients, ch)
	}
}
