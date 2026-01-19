package pubsub

import (
	"sync"

	"github.com/google/uuid"
)

// Hub manages pub/sub for GraphQL subscriptions
type Hub struct {
	subscriptions    map[uuid.UUID][]chan interface{}
	subscriptionsMux sync.RWMutex
}

// NewHub creates a new Hub
func NewHub() *Hub {
	return &Hub{
		subscriptions: make(map[uuid.UUID][]chan interface{}),
	}
}

// RegisterSubscription registers a subscription channel for a user
func (h *Hub) RegisterSubscription(userID uuid.UUID, ch chan interface{}) {
	h.subscriptionsMux.Lock()
	defer h.subscriptionsMux.Unlock()

	h.subscriptions[userID] = append(h.subscriptions[userID], ch)
}

// UnregisterSubscription removes a subscription channel for a user
func (h *Hub) UnregisterSubscription(userID uuid.UUID, ch chan interface{}) {
	h.subscriptionsMux.Lock()
	defer h.subscriptionsMux.Unlock()

	subs := h.subscriptions[userID]
	for i, sub := range subs {
		if sub == ch {
			h.subscriptions[userID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	// Clean up empty subscription list
	if len(h.subscriptions[userID]) == 0 {
		delete(h.subscriptions, userID)
	}
}

// BroadcastToUser sends an event to all subscribers for a user
func (h *Hub) BroadcastToUser(userID uuid.UUID, event interface{}) {
	h.subscriptionsMux.RLock()
	defer h.subscriptionsMux.RUnlock()

	if subs, ok := h.subscriptions[userID]; ok {
		for _, ch := range subs {
			select {
			case ch <- event:
			default:
				// Channel full, skip
			}
		}
	}
}
