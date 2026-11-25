//go:build integration

package pubcrawl

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"maps"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
)

// TestEmulatorMove exercises publish, failed move, retry, and pull against the Pub/Sub emulator.
func TestEmulatorMove(t *testing.T) {
	if os.Getenv("PUBSUB_EMULATOR_HOST") == "" {
		t.Fatal("set PUBSUB_EMULATOR_HOST to the running Pub/Sub emulator")
	}

	const project = "pubcrawl-integration"
	suffix := "-" + strings.ToLower(rand.Text())
	sourceTopic := "source" + suffix
	sourceSub := "source-sub" + suffix
	destTopic := "destination" + suffix
	destSub := "destination-sub" + suffix

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	client, err := pubsub.NewClient(ctx, project)

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})

	createEmulatorPair(t, ctx, client, project, sourceTopic, sourceSub)

	want := messageRecord{
		Data:        []byte{0xff, '\n', 0},
		Attributes:  map[string]string{"source": "test"},
		OrderingKey: "orders",
	}

	var input bytes.Buffer

	if err := json.NewEncoder(&input).Encode(want); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) (string, error) {
		var output bytes.Buffer
		cmd := NewCommand("test")
		cmd.SetArgs(append([]string{"--project", project}, args...))
		cmd.SetIn(&input)
		cmd.SetOut(&output)
		err := cmd.ExecuteContext(ctx)

		return output.String(), err
	}

	if _, err := run("publish", sourceTopic, "--format", "jsonl"); err != nil {
		t.Fatal(err)
	}

	if _, err := run("move", sourceSub, destTopic, "--limit", "1"); err == nil || !strings.Contains(err.Error(), "publish to") {
		t.Fatalf("move to missing topic returned %v, want publish error", err)
	}

	createEmulatorPair(t, ctx, client, project, destTopic, destSub)

	if _, err := run("move", sourceSub, destTopic, "--limit", "1", "--idle-timeout", "12s"); err != nil {
		t.Fatal(err)
	}

	output, err := run("pull", destSub, "--format", "jsonl", "--ack", "--limit", "1")

	if err != nil {
		t.Fatal(err)
	}

	var got messageRecord

	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("decode destination message %q: %v", output, err)
	}

	if !bytes.Equal(got.Data, want.Data) || !maps.Equal(got.Attributes, want.Attributes) || got.OrderingKey != want.OrderingKey {
		t.Fatalf("destination message = %+v, want %+v", got, want)
	}

	// The default ack deadline is 10 seconds. Wait beyond it so a leased,
	// unacknowledged message has time to become visible to another pull.
	if output, err := run("pull", sourceSub, "--format", "jsonl", "--limit", "1", "--idle-timeout", "12s"); err != nil || output != "" {
		t.Fatalf("source pull after move returned %q, %v, want empty output and no error", output, err)
	}
}


// createEmulatorPair creates a topic and subscription in project and registers their cleanup.
func createEmulatorPair(t *testing.T, ctx context.Context, client *pubsub.Client, project, topicID, subscriptionID string) {
	t.Helper()
	topic := "projects/" + project + "/topics/" + topicID
	subscription := "projects/" + project + "/subscriptions/" + subscriptionID

	if _, err := client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{Name: topic}); err != nil {
		t.Fatalf("create topic %s: %v", topic, err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.TopicAdminClient.DeleteTopic(ctx, &pubsubpb.DeleteTopicRequest{Topic: topic}); err != nil {
			t.Errorf("delete topic %s: %v", topic, err)
		}
	})

	if _, err := client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
		Name: subscription, Topic: topic, EnableMessageOrdering: true,
	}); err != nil {
		t.Fatalf("create subscription %s: %v", subscription, err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := client.SubscriptionAdminClient.DeleteSubscription(ctx, &pubsubpb.DeleteSubscriptionRequest{Subscription: subscription}); err != nil {
			t.Errorf("delete subscription %s: %v", subscription, err)
		}
	})
}
