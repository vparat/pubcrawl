package pubcrawl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"cloud.google.com/go/pubsub/v2"
	"github.com/spf13/cobra"
)


// newPullCommand returns the pull command using the root project flag.
func newPullCommand(project *string) *cobra.Command {
	var options receiveOptions
	var acknowledge, legacyAck bool
	var output, format string

	var subscription string

	cmd := &cobra.Command{
		Use:     "pull SUBSCRIPTION",
		Aliases: []string{"drain"},
		Short:   "Write subscription messages to stdout or a file",
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			var err error

			subscription, err = resourceName(*project, "subscriptions", args[0])

			if err != nil {
				return err
			}

			if err := options.validate(); err != nil {
				return err
			}

			return validateFormat(format)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ack := acknowledge || legacyAck
			settings := options
			settings.limit = pullLimit(options.limit, ack, cmd.Flags().Changed("limit"))

			return withClient(cmd.Context(), *project, func(client *pubsub.Client) (err error) {
				writer := cmd.OutOrStdout()
				var file *os.File

				if output != "" {
					file, err = os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)

					if err != nil {
						return fmt.Errorf("create output file: %w", err)
					}

					defer func() { err = errors.Join(err, file.Close()) }()
					writer = file
				}

				return receive(cmd.Context(), client.Subscriber(subscription), settings, ack, func(_ context.Context, message *pubsub.Message) error {
					if err := writeMessage(writer, format, message); err != nil {
						return err
					}

					if ack && file != nil {
						return file.Sync()
					}

					return nil
				})
			})
		},
	}

	options.bind(cmd, 1000)
	cmd.Flags().Lookup("limit").Usage = "maximum messages (unlimited by default with --ack; -1 reads until idle)"
	cmd.Flags().BoolVarP(&acknowledge, "ack", "a", false, "acknowledge messages after writing them")
	cmd.Flags().BoolVar(&legacyAck, "auto-ack", false, "alias for --ack")
	cmd.Flags().StringVarP(&output, "to-file", "f", "", "create a new output file")
	cmd.Flags().StringVar(&format, "format", "lines", "message format (lines or jsonl)")

	return cmd
}


func pullLimit(limit int, acknowledge, explicit bool) int {
	if acknowledge && !explicit {
		return -1
	}
	return limit
}


// writeMessage writes message to writer in the requested format and returns any encoding or write error.
func writeMessage(writer io.Writer, format string, message *pubsub.Message) error {
	if format == "jsonl" {
		return json.NewEncoder(writer).Encode(messageRecord{
			Data:        message.Data,
			Attributes:  message.Attributes,
			OrderingKey: message.OrderingKey,
		})
	}

	if bytes.Contains(message.Data, []byte{'\n'}) || bytes.HasSuffix(message.Data, []byte{'\r'}) {
		return errors.New("payload cannot round-trip as a single line; use --format jsonl")
	}

	_, err := fmt.Fprintf(writer, "%s\n", message.Data)

	return err
}
