// Package events replaces desktop events with a process-local, concurrency-safe bus.
package events

import (
	"context"
	"sync"
)

type Event struct {
	Name string `json:"name"`
	Data []any  `json:"data"`
}
type Bus struct {
	mu       sync.Mutex
	next     int
	clients  map[int]chan Event
	handlers map[string]map[int]func(...any)
}
type key struct{}

func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, key{}, &Bus{clients: make(map[int]chan Event), handlers: make(map[string]map[int]func(...any))})
}
func bus(ctx context.Context) *Bus { return ctx.Value(key{}).(*Bus) }
func Subscribe(ctx context.Context) (<-chan Event, func()) {
	b := bus(ctx)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.next++
	id := b.next
	ch := make(chan Event, 256)
	b.clients[id] = ch
	return ch, func() { b.mu.Lock(); delete(b.clients, id); b.mu.Unlock() }
}
func EventsEmit(ctx context.Context, name string, data ...any) {
	b := bus(ctx)
	b.mu.Lock()
	for _, ch := range b.clients {
		select {
		case ch <- Event{name, data}:
		default:
		}
	}
	callbacks := []func(...any){}
	for _, fn := range b.handlers[name] {
		callbacks = append(callbacks, fn)
	}
	b.mu.Unlock()
	for _, fn := range callbacks {
		fn(data...)
	}
}
func EventsOn(ctx context.Context, name string, fn func(...any)) func() {
	b := bus(ctx)
	b.mu.Lock()
	b.next++
	id := b.next
	if b.handlers[name] == nil {
		b.handlers[name] = make(map[int]func(...any))
	}
	b.handlers[name][id] = fn
	b.mu.Unlock()
	return func() { b.mu.Lock(); delete(b.handlers[name], id); b.mu.Unlock() }
}
func EventsOff(ctx context.Context, name string) {
	b := bus(ctx)
	b.mu.Lock()
	delete(b.handlers, name)
	b.mu.Unlock()
}
