package adapters

import "context"

type Message struct {
	SenderID string
	Text     string
}

type Adapter interface {
	ID() string
	Start(ctx context.Context, onMessage func(Message)) error
	Send(ctx context.Context, target string, text string) error
}

type TypingSender interface {
	SendTyping(ctx context.Context, target string) error
}

type Registry struct {
	adapters map[string]Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: map[string]Adapter{}}
}

func (r *Registry) Register(adapter Adapter) {
	r.adapters[adapter.ID()] = adapter
}

func (r *Registry) Get(id string) Adapter {
	return r.adapters[id]
}
