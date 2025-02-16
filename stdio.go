package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"strings"
)

// StdIO implements a standard input/output transport layer for MCP communication using
// JSON-RPC message encoding over stdin/stdout or similar io.Reader/io.Writer pairs. It
// provides a single persistent session identified as "1" and handles bidirectional message
// passing through internal channels, processing messages sequentially.
//
// The transport layer maintains internal state through its embedded stdIOSession and can
// be used as either ServerTransport or ClientTransport. Proper initialization requires
// using the NewStdIO constructor function to create new instances.
//
// Resources must be properly released by calling Close when the StdIO instance is no
// longer needed.
type StdIO struct {
	sess stdIOSession
}

type stdIOSession struct {
	reader io.Reader
	writer io.Writer
	logger *slog.Logger
}

// NewStdIO creates a new StdIO instance configured with the provided reader and writer.
// The instance is initialized with default logging and required internal communication
// channels.
func NewStdIO(reader io.Reader, writer io.Writer) StdIO {
	return StdIO{
		sess: stdIOSession{
			reader: reader,
			writer: writer,
			logger: slog.Default(),
		},
	}
}

// Sessions implements the ServerTransport interface by providing an iterator that yields
// a single persistent session. This session remains active throughout the lifetime of
// the StdIO instance.
func (s StdIO) Sessions() iter.Seq[Session] {
	return func(yield func(Session) bool) {
		yield(s.sess)
	}
}

// Send implements the ClientTransport interface by transmitting a JSON-RPC message to
// the server through the established session. The context can be used to control the
// transmission timeout.
func (s StdIO) Send(ctx context.Context, msg JSONRPCMessage) error {
	return s.sess.Send(ctx, msg)
}

// StartSession implements the ClientTransport interface by initializing a new session
// and returning an iterator for receiving server messages. The ready channel is closed
// immediately to indicate session establishment.
func (s StdIO) StartSession(_ context.Context, ready chan<- error) (iter.Seq[JSONRPCMessage], error) {
	close(ready)
	return s.sess.Messages(), nil
}

func (s stdIOSession) ID() string {
	return "1"
}

func (s stdIOSession) Send(ctx context.Context, msg JSONRPCMessage) error {
	msgBs, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	// We append newline to maintain message framing protocol
	msgBs = append(msgBs, '\n')

	errs := make(chan error, 1)

	// We use a goroutine for writing to prevent blocking on slow writers
	// while still respecting context cancellation
	go func() {
		_, err = s.writer.Write(msgBs)
		if err != nil {
			errs <- fmt.Errorf("failed to write message: %w", err)
			return
		}
		errs <- nil
	}()

	// We prioritize context cancellation and session termination over write completion
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errs:
		return err
	}
}

func (s stdIOSession) Messages() iter.Seq[JSONRPCMessage] {
	return func(yield func(JSONRPCMessage) bool) {
		// Use bufio.Reader instead of bufio.Scanner to avoid max token size errors.
		reader := bufio.NewReader(s.reader)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				s.logger.Error("failed to read message", "err", err)
				return
			}
			line = strings.TrimSuffix(line, "\n")
			if line == "" {
				continue
			}

			var msg JSONRPCMessage
			err = json.Unmarshal([]byte(line), &msg)
			if err != nil {
				s.logger.Error("failed to unmarshal message", "err", err)
				continue
			}

			// We stop iteration if yield returns false
			if !yield(msg) {
				return
			}
		}
	}
}
