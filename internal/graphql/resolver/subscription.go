package resolver

import (
	"context"

	"github.com/user/remind-me/backend/internal/graphql/middleware"
	"github.com/user/remind-me/backend/internal/graphql/model"
	apperrors "github.com/user/remind-me/backend/pkg/errors"
)

// ReminderChanged returns a channel that receives reminder change events
func (r *Resolver) ReminderChanged(ctx context.Context) (<-chan *model.ReminderChangeEvent, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, apperrors.ErrUnauthorized
	}

	// Create a channel for this subscription
	eventChan := make(chan *model.ReminderChangeEvent, 10)

	// Create an internal channel to receive generic events from the hub
	hubChan := make(chan interface{}, 10)

	// Register with the WebSocket hub
	if r.Hub != nil {
		r.Hub.RegisterSubscription(userID, hubChan)
	}

	// Start a goroutine to convert hub events to typed events
	go func() {
		defer close(eventChan)
		defer func() {
			if r.Hub != nil {
				r.Hub.UnregisterSubscription(userID, hubChan)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-hubChan:
				if !ok {
					return
				}
				// Type assert to our event type
				if reminderEvent, ok := event.(*model.ReminderChangeEvent); ok {
					select {
					case eventChan <- reminderEvent:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return eventChan, nil
}
