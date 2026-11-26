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

func TestPullLimit(t *testing.T) {
	project := "test"
	cmd := newPullCommand(&project)
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name          string
		value         int
		ack, explicit bool
		want          int
	}{
		{"preview", limit, false, false, 1000},
		{"ack", limit, true, false, -1},
		{"unlimited preview", -1, false, true, -1},
		{"bounded ack", 3, true, true, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pullLimit(tc.value, tc.ack, tc.explicit); got != tc.want {
				t.Errorf("limit = %d, want %d", got, tc.want)
			}
		})
	}
}
