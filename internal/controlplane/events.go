package controlplane

import (
	"context"
	"sync"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/state"
)

type EventStore interface {
	PersistEvent(ctx context.Context, event contract.EventEnvelope) (contract.EventEnvelope, error)
	ListEvents(ctx context.Context, opts state.EventListOptions) ([]contract.EventEnvelope, error)
	EventSequenceForID(ctx context.Context, runID, eventID string) (int64, error)
}

type Publisher struct {
	store      EventStore
	queueLimit int
	mu         sync.Mutex
	nextID     int
	clients    map[int]chan contract.EventEnvelope
}

func NewPublisher(store EventStore, queueLimit int) *Publisher {
	if queueLimit <= 0 {
		queueLimit = 1000
	}
	return &Publisher{store: store, queueLimit: queueLimit, clients: map[int]chan contract.EventEnvelope{}}
}

func (p *Publisher) Publish(ctx context.Context, event contract.EventEnvelope) (contract.EventEnvelope, error) {
	stored, err := p.store.PersistEvent(ctx, event)
	if err != nil {
		return contract.EventEnvelope{}, err
	}
	p.broadcast(stored)
	return stored, nil
}

func (p *Publisher) Subscribe() (int, <-chan contract.EventEnvelope, func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nextID++
	id := p.nextID
	ch := make(chan contract.EventEnvelope, p.queueLimit)
	p.clients[id] = ch
	return id, ch, func() { p.unsubscribe(id) }
}

func (p *Publisher) broadcast(event contract.EventEnvelope) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, ch := range p.clients {
		select {
		case ch <- event:
		default:
			close(ch)
			delete(p.clients, id)
		}
	}
}

func (p *Publisher) unsubscribe(id int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ch, ok := p.clients[id]; ok {
		close(ch)
		delete(p.clients, id)
	}
}
