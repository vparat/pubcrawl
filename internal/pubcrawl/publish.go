package pubcrawl

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"cloud.google.com/go/pubsub/v2"
	"github.com/spf13/cobra"
)


// messageRecord is the JSON Lines representation of publishable message fields.
type messageRecord struct {
	Data        []byte            `json:"data"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	OrderingKey string            `json:"ordering_key,omitempty"`
}


// newPublishCommand returns the publish command using the root project flag.
func newPublishCommand(project *string) *cobra.Command {
	var input, literal, format string

	var topic string

	cmd := &cobra.Command{
		Use:     "publish TOPIC",
		Aliases: []string{"shove"},
		Short:   "Publish messages from stdin, a file, or a literal payload",
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Keep flag conflicts ahead of resource and format errors.
			if err := cmd.ValidateFlagGroups(); err != nil {
				return err
			}

			var err error

			topic, err = resourceName(*project, "topics", args[0])

			if err != nil {
				return err
			}

			return validateFormat(format)
		},
		RunE: func(cmd *cobra.Command, _ []string) (runErr error) {
			reader := cmd.InOrStdin()

			if cmd.Flags().Changed("from-file") {
				file, err := os.Open(input)

				if err != nil {
					return fmt.Errorf("open input file: %w", err)
				}

				defer func() { runErr = errors.Join(runErr, file.Close()) }()
				reader = file
			}

			return withClient(cmd.Context(), *project, func(client *pubsub.Client) error {
				publisher := client.Publisher(topic)
				publisher.EnableMessageOrdering = true
				defer publisher.Stop()

				send := func(message *pubsub.Message) error {
					return publish(cmd.Context(), publisher, message)
				}

				if cmd.Flags().Changed("message") {
					return send(&pubsub.Message{Data: []byte(literal)})
				}

				return publishRecords(reader, format, send)
			})
		},
	}

	cmd.Flags().StringVarP(&input, "from-file", "f", "", "read messages from a file")
	cmd.Flags().StringVarP(&literal, "message", "m", "", "publish a single literal payload")
	cmd.Flags().StringVar(&format, "format", "lines", "input format for stdin or a file (lines or jsonl)")
	cmd.MarkFlagsMutuallyExclusive("message", "from-file")
	cmd.MarkFlagsMutuallyExclusive("message", "format")

	return cmd
}


// publishRecords reads messages from reader, calls send for each, and returns the first error.
func publishRecords(reader io.Reader, format string, send func(*pubsub.Message) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	line := 0

	for scanner.Scan() {
		line++
		record := messageRecord{Data: scanner.Bytes()}

		if format == "jsonl" {
			record = messageRecord{}

			if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
				return fmt.Errorf("decode line %d: %w", line, err)
			}
		}

		message := &pubsub.Message{Data: record.Data, Attributes: record.Attributes, OrderingKey: record.OrderingKey}

		if err := send(message); err != nil {
			return fmt.Errorf("line %d: %w", line, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read line %d: %w", line+1, err)
	}

	return nil
}


// validateFormat returns an error if format does not identify a supported message encoding.
func validateFormat(format string) error {
	switch format {
	case "lines", "jsonl":
		return nil
	default:
		return fmt.Errorf("invalid --format %q; expected lines or jsonl", format)
	}
}


// publish waits for publisher to confirm message and returns any publish error.
func publish(ctx context.Context, publisher *pubsub.Publisher, message *pubsub.Message) error {
	// Received messages include server-assigned fields and an ack handler.
	outgoing := &pubsub.Message{
		Data:        message.Data,
		Attributes:  message.Attributes,
		OrderingKey: message.OrderingKey,
	}

	if _, err := publisher.Publish(ctx, outgoing).Get(ctx); err != nil {
		return fmt.Errorf("publish to %s: %w", publisher, err)
	}

	return nil
}
