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
	sess   stdIOSession
	closed chan struct{}
}

type stdIOSession struct {
	reader io.Reader
	writer io.Writer
	logger *slog.Logger

	done   chan struct{}
	closed chan struct{}
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
			done:   make(chan struct{}),
			closed: make(chan struct{}),
		},
		closed: make(chan struct{}),
	}
}

// Sessions implements the ServerTransport interface by providing an iterator that yields
// a single persistent session. This session remains active throughout the lifetime of
// the StdIO instance.
func (s StdIO) Sessions() iter.Seq[Session] {
	return func(yield func(Session) bool) {
		defer close(s.closed)

		// StdIO only supports a single session, so we yield it and wait until it's done.
		yield(s.sess)
		<-s.sess.done
	}
}

// Shutdown implements the ServerTransport interface by closing the session.
func (s StdIO) Shutdown(context.Context) error {
	// We don't need to do anything here, since StdIO is only supports a single session,
	// and we leave it at the stopping of session.
	return nil
}

// Send implements the ClientTransport interface by transmitting a JSON-RPC message to
// the server through the established session. The context can be used to control the
// transmission timeout.
func (s StdIO) Send(_ context.Context, msg JSONRPCMessage) error {
	return s.sess.Send(msg)
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

func (s stdIOSession) Send(msg JSONRPCMessage) error {
	msgBs, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	// Append newline to maintain message framing protocol
	msgBs = append(msgBs, '\n')

	errs := make(chan error, 1)

	// Spawn a goroutine for writing to prevent blocking on slow writers
	// while still respecting context cancellation or done channel.
	go func() {
		// Safeguard to make sure the session is still alive
		select {
		case <-s.done:
			s.logger.Warn("session is closed while writing message", slog.String("message", string(msgBs)))
			return
		default:
		}

		_, err = s.writer.Write(msgBs)
		if err != nil {
			errs <- fmt.Errorf("failed to write message: %w", err)
			return
		}
		errs <- nil
	}()

	select {
	case err := <-errs:
		if err != nil {
			s.logger.Error("failed to send message", slog.String("err", err.Error()))
		}
		return err
	case <-s.done:
		s.logger.Warn("session is closed while sending message", slog.String("message", string(msgBs)))
		return nil
	}
}

func (s stdIOSession) Messages() iter.Seq[JSONRPCMessage] {
	return func(yield func(JSONRPCMessage) bool) {
		defer close(s.closed)

		// Use bufio.Reader instead of bufio.Scanner to avoid max token size errors.
		reader := bufio.NewReader(s.reader)
		for {
			type lineWithErr struct {
				line string
				err  error
			}

			lines := make(chan lineWithErr)

			// We use goroutines to avoid blocking on slow readers, so we can listen
			// to done channel and return if needed.
			go func() {
				line, err := reader.ReadString('\n')
				if err != nil {
					lines <- lineWithErr{err: err}
					return
				}
				lines <- lineWithErr{line: strings.TrimSuffix(line, "\n")}
			}()

			var lwe lineWithErr
			select {
			case <-s.done:
				return
			case lwe = <-lines:
			}

			if lwe.err != nil {
				if errors.Is(lwe.err, io.EOF) {
					return
				}
				s.logger.Error("failed to read message", "err", lwe.err)
				return
			}

			if lwe.line == "" {
				continue
			}

			var msg JSONRPCMessage
			if err := json.Unmarshal([]byte(lwe.line), &msg); err != nil {
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

func (s stdIOSession) Stop() {
	close(s.done)
	<-s.closed
}
