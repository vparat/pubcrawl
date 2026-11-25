package pubcrawl

import (
	"bytes"
	"maps"
	"testing"

	"cloud.google.com/go/pubsub/v2"
)

// TestJSONLRoundTrip verifies that multiline binary data and metadata survive export and import.
func TestJSONLRoundTrip(t *testing.T) {
	want := &pubsub.Message{Data: []byte{0xff, '\n', '\r'}, Attributes: map[string]string{"reason": "retry"}, OrderingKey: "orders"}
	var buffer bytes.Buffer

	if err := writeMessage(&buffer, "jsonl", want); err != nil {
		t.Fatal(err)
	}

	calls := 0
	err := publishRecords(&buffer, "jsonl", func(got *pubsub.Message) error {
		calls++

		if !bytes.Equal(got.Data, want.Data) || !maps.Equal(got.Attributes, want.Attributes) || got.OrderingKey != want.OrderingKey {
			t.Fatalf("round-trip message = %+v, want %+v", got, want)
		}

		return nil
	})

	if err != nil || calls != 1 {
		t.Fatalf("publishRecords returned %v after %d calls", err, calls)
	}
}
