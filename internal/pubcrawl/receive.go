package pubcrawl

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/pubsub/v2"
)


// receive processes messages serially with handle and optionally acknowledges successful
// deliveries. It returns after the limit, an idle interval, cancellation, or an error.
func receive(ctx context.Context, sub *pubsub.Subscriber, options receiveOptions, acknowledge bool, handle func(context.Context, *pubsub.Message) error) error {
	receiveCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sub.ReceiveSettings.NumGoroutines = 1
	sub.ReceiveSettings.MaxOutstandingMessages = 1

	var mu sync.Mutex
	var count int
	var handleErr error

	// Previewed messages can be redelivered as soon as they are nacked.
	var seen map[string]struct{}

	if !acknowledge {
		seen = make(map[string]struct{})
	}

	idleDeadline := time.Now().Add(options.idle)
	// The timer and handler share the latest idle deadline. A timer that
	// expires during message handling rechecks the deadline after the handler.
	var idleTimer *time.Timer
	mu.Lock()
	idleTimer = time.AfterFunc(options.idle, func() {
		mu.Lock()
		defer mu.Unlock()

		if receiveCtx.Err() != nil {
			return
		}

		if remaining := time.Until(idleDeadline); remaining > 0 {
			idleTimer.Reset(remaining)

			return
		}

		cancel()
	})
	mu.Unlock()

	err := sub.Receive(receiveCtx, func(messageCtx context.Context, message *pubsub.Message) {
		mu.Lock()
		defer mu.Unlock()

		if receiveCtx.Err() != nil {
			message.Nack()

			return
		}

		if _, duplicate := seen[message.ID]; duplicate {
			message.Nack()

			return
		}

		if err := handle(messageCtx, message); err != nil {
			handleErr = fmt.Errorf("process message %s: %w", message.ID, err)
			message.Nack()
			cancel()

			return
		}

		if acknowledge {
			if _, err := message.AckWithResult().Get(messageCtx); err != nil {
				handleErr = fmt.Errorf("acknowledge message %s: %w", message.ID, err)
				cancel()

				return
			}
		} else {
			seen[message.ID] = struct{}{}
			message.Nack()
		}

		count++

		if count == options.limit {
			cancel()

			return
		}

		idleDeadline = time.Now().Add(options.idle)

		if idleTimer.Stop() {
			idleTimer.Reset(options.idle)
		}
	})

	cancel()
	mu.Lock()
	idleTimer.Stop()
	mu.Unlock()

	return errors.Join(handleErr, err, ctx.Err())
}
