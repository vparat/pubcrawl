package pubcrawl

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"cloud.google.com/go/pubsub/v2"
)

func TestPublishPacing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		start := time.Now()
		var sent []time.Duration
		send := pacedSender(ctx, 2, func(*pubsub.Message) error {
			sent = append(sent, time.Since(start))
			return nil
		})
		for range 2 {
			if err := send(&pubsub.Message{}); err != nil {
				t.Fatal(err)
			}
		}
		time.AfterFunc(time.Millisecond, cancel)
		if err := send(&pubsub.Message{}); !errors.Is(err, context.Canceled) {
			t.Fatalf("waiting send returned %v", err)
		}
		if len(sent) != 2 || sent[0] != 0 || sent[1] != 500*time.Millisecond {
			t.Fatalf("send times = %v", sent)
		}
	})
}
