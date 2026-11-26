//go:build unix

package pubcrawl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"golang.org/x/sys/unix"
)

func TestCancelPublishInput(t *testing.T) {
	for _, source := range []string{"stdin pipe", "FIFO without writer"} {
		t.Run(source, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
			defer cancel()
			var input *os.File
			var err error
			if source == "stdin pipe" {
				var writer *os.File
				input, writer, err = os.Pipe()
				if err == nil {
					defer writer.Close()
				}
			} else {
				path := filepath.Join(t.TempDir(), "input")
				if err := unix.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
				input, err = openPublishFile(path)
			}
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			err = publishInput(ctx, input, "lines", func(*pubsub.Message) error {
				return errors.New("unexpected message")
			})
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("read returned %v", err)
			}
		})
	}
}
