//go:build unix

package pubcrawl

import (
	"context"
	"io"
	"os"
	"syscall"

	"cloud.google.com/go/pubsub/v2"
	"golang.org/x/sys/unix"
)

func openPublishFile(path string) (*os.File, error) {
	// A FIFO with no writer must reach the cancellable read loop.
	return os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
}

func publishInput(ctx context.Context, reader io.Reader, format string, send func(*pubsub.Message) error) error {
	if file, ok := reader.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			raw, err := file.SyscallConn()
			if err != nil {
				return err
			}
			reader = &pollReader{ctx: ctx, raw: raw}
		}
	}
	return publishRecords(reader, format, send)
}

// pollReader bounds cancellation latency while leaving the caller's descriptor flags intact.
type pollReader struct {
	ctx context.Context
	raw syscall.RawConn
}

func (r *pollReader) Read(p []byte) (int, error) {
	var n int
	var readErr error
	for {
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
		err := r.raw.Control(func(fd uintptr) {
			fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
			n, readErr = unix.Poll(fds, 100)
			if n > 0 && readErr == nil {
				n, readErr = unix.Read(int(fd), p)
				if n == 0 && readErr == nil {
					readErr = io.EOF
				}
			}
		})
		if err != nil {
			return 0, err
		}
		if readErr == unix.EINTR || readErr == unix.EAGAIN {
			continue
		}
		if readErr != nil {
			return 0, readErr
		}
		if n > 0 {
			return n, nil
		}
	}
}
