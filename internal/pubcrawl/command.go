package pubcrawl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/spf13/cobra"
)


// NewCommand returns a fresh CLI that reports the supplied version.
func NewCommand(version string) *cobra.Command {
	var project string
	root := &cobra.Command{
		Use:           "pubcrawl",
		Short:         "Pull, publish, and move Google Cloud Pub/Sub messages",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.PersistentFlags().StringVarP(&project, "project", "p", "", "Google Cloud project ID (required; used for unqualified resource IDs)")
	root.AddCommand(newPullCommand(&project), newPublishCommand(&project), newMoveCommand(&project))

	return root
}


// withClient runs operation against project and returns any operation or cleanup error.
func withClient(ctx context.Context, project string, operation func(*pubsub.Client) error) (err error) {
	client, err := pubsub.NewClient(ctx, project)

	if err != nil {
		return fmt.Errorf("create Pub/Sub client: %w", err)
	}

	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close Pub/Sub client: %w", closeErr))
		}
	}()

	return operation(client)
}


// resourceName expands id using project and kind or validates a qualified name.
func resourceName(project, kind, id string) (string, error) {
	if project == "" || strings.ContainsAny(project, "/ \t\r\n") {
		return "", errors.New("--project must be a Google Cloud project ID")
	}

	if id == "" || strings.ContainsAny(id, " \t\r\n") {
		return "", fmt.Errorf("invalid %s resource %q", kind, id)
	}

	parts := strings.Split(id, "/")

	switch {
	case len(parts) == 1:
		return fmt.Sprintf("projects/%s/%s/%s", project, kind, id), nil
	case len(parts) == 4 && parts[0] == "projects" && parts[1] != "" && parts[2] == kind && parts[3] != "":
		return id, nil
	default:
		return "", fmt.Errorf("expected a resource ID or projects/PROJECT/%s/ID, got %q", kind, id)
	}
}


// receiveOptions holds the stopping conditions for a subscription read.
type receiveOptions struct {
	limit int
	idle  time.Duration
}


// bind registers receive flags on cmd.
func (o *receiveOptions) bind(cmd *cobra.Command) {
	cmd.Flags().IntVarP(&o.limit, "limit", "l", -1, "maximum messages to process (-1 reads until idle)")
	cmd.Flags().DurationVar(&o.idle, "idle-timeout", 5*time.Second, "stop after this long without a new message")
}


// validate returns an error for unsupported receive settings.
func (o receiveOptions) validate() error {
	if o.limit != -1 && o.limit < 1 {
		return errors.New("--limit must be positive or -1")
	}

	if o.idle <= 0 {
		return errors.New("--idle-timeout must be positive")
	}

	return nil
}
