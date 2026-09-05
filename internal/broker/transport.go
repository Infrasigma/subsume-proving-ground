package broker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	MaxTransportFrame  = 4 << 20
	MaxTransportWindow = 5 * time.Second
)

var (
	ErrTransportFrameTooLarge = errors.New("transport frame exceeds 4 MiB limit")
	ErrTransportTrailingData = errors.New("transport request contains trailing data")
)

// HandleConnection is the untrusted transport edge. It owns only bytes,
// framing, deadlines, JSON syntax, and connection teardown. All action
// authority remains inside Broker.Execute.
func (b *Broker) HandleConnection(conn net.Conn) error {
	defer conn.Close()

	if conn == nil {
		return errors.New("nil transport connection")
	}
	if err := conn.SetDeadline(time.Now().Add(MaxTransportWindow)); err != nil {
		return fmt.Errorf("set transport deadline: %w", err)
	}

	limited := &countingReader{Reader: io.LimitReader(conn, MaxTransportFrame+1)}
	reader := bufio.NewReaderSize(limited, 64<<10)
	var envelope json.RawMessage
	dec := json.NewDecoder(reader)
	if err := dec.Decode(&envelope); err != nil {
		if limited.n > MaxTransportFrame {
			return ErrTransportFrameTooLarge
		}
		return fmt.Errorf("invalid transport JSON: %w", err)
	}

	// A request is exactly one JSON envelope. If another complete value is
	// already buffered, reject it before any authority-bearing code runs.
	if reader.Buffered() > 0 {
		var extra any
		if err := dec.Decode(&extra); err == nil {
			return ErrTransportTrailingData
		}
	}

	receipt, execErr := b.Execute(context.Background(), envelope)
	if err := conn.SetWriteDeadline(time.Now().Add(MaxTransportWindow)); err != nil {
		return fmt.Errorf("set transport write deadline: %w", err)
	}
	response, err := json.Marshal(receipt)
	if err != nil {
		return fmt.Errorf("encode receipt: %w", err)
	}
	if _, err := conn.Write(response); err != nil {
		return fmt.Errorf("write receipt: %w", err)
	}
	if execErr != nil {
		return execErr
	}
	return nil
}

// Serve accepts untrusted local connections. Each connection gets its own
// goroutine; HandleConnection contains no shared mutable connection state.
func (b *Broker) Serve(l net.Listener) error {
	if l == nil {
		return errors.New("nil transport listener")
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go func() {
			_ = b.HandleConnection(conn)
		}()
	}
}

type countingReader struct {
	io.Reader
	n int
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.n += n
	return n, err
}
