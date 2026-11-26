//go:build !unix

package pubcrawl

import (
	"context"
	"io"
	"os"
	"time"

	"cloud.google.com/go/pubsub/v2"
)

func openPublishFile(path string) (*os.File, error) { return os.Open(path) }

func publishInput(ctx context.Context, reader io.Reader, format string, send func(*pubsub.Message) error) error {
	if file, ok := reader.(*os.File); ok {
		done := make(chan struct{})
		stop := context.AfterFunc(ctx, func() {
			defer close(done)
			file.SetReadDeadline(time.Now())
		})
		defer func() {
			if !stop() {
				<-done
				file.SetReadDeadline(time.Time{})
			}
		}()
	}
	err := publishRecords(reader, format, send)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
