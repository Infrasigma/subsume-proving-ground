package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	MaxTransportFrame   = 4 << 20
	MaxTransportWindow  = 5 * time.Second
)

var (
	ErrTransportFrameTooLarge = errors.New("transport frame exceeds 4 MiB limit")
	ErrTransportTrailingData  = errors.New("transport request contains trailing data")
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

	limited := io.LimitReader(conn, MaxTransportFrame+1)
	request, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("read transport request: %w", err)
	}
	if len(request) > MaxTransportFrame {
		return ErrTransportFrameTooLarge
	}
	if len(request) == 0 {
		return errors.New("empty transport request")
	}

	var envelope json.RawMessage
	dec := json.NewDecoder(bytesReader(request))
	if err := dec.Decode(&envelope); err != nil {
		return fmt.Errorf("invalid transport JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return ErrTransportTrailingData
		}
		return fmt.Errorf("invalid trailing transport data: %w", err)
	}

	receipt, execErr := b.Execute(context.Background(), envelope)
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

// bytesReader is deliberately tiny so the transport remains dependent only on
// standard-library byte/JSON primitives.
func bytesReader(p []byte) io.Reader {
	return &sliceReader{p: p}
}

type sliceReader struct {
	p []byte
	i int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.i >= len(r.p) {
		return 0, io.EOF
	}
	n := copy(p, r.p[r.i:])
	r.i += n
	return n, nil
}
