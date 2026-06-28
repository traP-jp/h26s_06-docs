package main

import "sync"

const clientEventQueueSize = 64

type eventHub struct {
	mu      sync.RWMutex
	closed  bool
	clients map[chan sseEvent]struct{}
}

type viewerSignal struct {
	ChannelID string
}

type viewerSignalHub struct {
	mu      sync.RWMutex
	closed  bool
	clients map[chan viewerSignal]struct{}
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

func (h *eventHub) hasSubscribers() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return !h.closed && len(h.clients) > 0
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

func newViewerSignalHub() *viewerSignalHub {
	return &viewerSignalHub{clients: map[chan viewerSignal]struct{}{}}
}

func (h *viewerSignalHub) subscribe() chan viewerSignal {
	ch := make(chan viewerSignal, clientEventQueueSize)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(ch)
		return ch
	}
	h.clients[ch] = struct{}{}
	return ch
}

func (h *viewerSignalHub) unsubscribe(ch chan viewerSignal) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
}

func (h *viewerSignalHub) publish(signal viewerSignal) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.closed {
		return
	}
	for ch := range h.clients {
		select {
		case ch <- signal:
		default:
		}
	}
}

func (h *viewerSignalHub) close() {
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
