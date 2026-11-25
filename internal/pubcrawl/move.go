package pubcrawl

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/spf13/cobra"
)


// newMoveCommand returns the move command using the root project flag.
func newMoveCommand(project *string) *cobra.Command {
	var options receiveOptions

	var source, destination string

	cmd := &cobra.Command{
		Use:   "move SUBSCRIPTION TOPIC",
		Short: "Publish subscription messages to a topic, then acknowledge them",
		Args:  cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var err error

			source, err = resourceName(*project, "subscriptions", args[0])

			if err != nil {
				return err
			}

			destination, err = resourceName(*project, "topics", args[1])

			if err != nil {
				return err
			}

			return options.validate()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withClient(cmd.Context(), *project, func(client *pubsub.Client) error {
				subscription, err := client.SubscriptionAdminClient.GetSubscription(cmd.Context(), &pubsubpb.GetSubscriptionRequest{Subscription: source})

				if err != nil {
					return fmt.Errorf("get source subscription: %w", err)
				}

				if subscription.GetTopic() == destination {
					return errors.New("destination topic feeds the source subscription")
				}

				publisher := client.Publisher(destination)
				publisher.EnableMessageOrdering = true
				defer publisher.Stop()

				return receive(cmd.Context(), client.Subscriber(source), options, true, func(ctx context.Context, message *pubsub.Message) error {
					return publish(ctx, publisher, message)
				})
			})
		},
	}

	options.bind(cmd)

	return cmd
}
