package broker

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/ledger"
	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
	"github.com/Infrasigma/subsume-proving-ground/internal/provider"
)

type captureTransportVerifier struct {
	contract provider.ActionContract
	called   bool
	raw      []byte
}

func (v *captureTransportVerifier) Verify(raw []byte) (provider.ActionContract, error) {
	v.called = true
	v.raw = append([]byte(nil), raw...)
	return v.contract, nil
}

func newTransportTestBroker(t *testing.T, verifier Verifier, providerResult any) *Broker {
	t.Helper()
	l, err := ledger.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := &fakeProvider{result: providerResult}
	e := &fakeEvidence{result: map[string]any{"verified": true}}
	b, err := New(verifier, l, p, e, "broker-transport-test", "postgresql", "boundary-transport-test", key)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// shortDeadlineConn keeps the production deadline logic intact while making
// the slow-loris test fast and deterministic.
type shortDeadlineConn struct {
	net.Conn
	window time.Duration
}

func (c *shortDeadlineConn) SetDeadline(_ time.Time) error {
	return c.Conn.SetDeadline(time.Now().Add(c.window))
}

func TestHandleConnectionHappyPathReturnsSignedReceiptAndExactEnvelope(t *testing.T) {
	contract := brokerTestContract()
	verifier := &captureTransportVerifier{contract: contract}
	b := newTransportTestBroker(t, verifier, map[string]any{"active": false})

	server, client := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- b.HandleConnection(server) }()

	expectedRequest := []byte(`{"action":"deactivate","id":"1842"}`)
	if _, err := client.Write(expectedRequest); err != nil {
		t.Fatal(err)
	}

	var got protocol.Receipt
	if err := json.NewDecoder(client).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Type != "Receipt" || got.SignerID == "" || got.Signature == "" {
		t.Fatalf("invalid signed receipt envelope: %+v", got)
	}
	if !bytes.Equal(verifier.raw, expectedRequest) {
		t.Fatalf("broker received %q, want exact %q", verifier.raw, expectedRequest)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestHandleConnectionOverflowNeverInvokesBrokerExecute(t *testing.T) {
	verifier := &captureTransportVerifier{contract: brokerTestContract()}
	b := newTransportTestBroker(t, verifier, map[string]any{"active": false})

	server, client := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- b.HandleConnection(server) }()

	payload := `{"x":"` + strings.Repeat("a", MaxTransportFrame) + `"}`
	writeDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(client, strings.NewReader(payload))
		writeDone <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, ErrTransportFrameTooLarge) {
			t.Fatalf("error = %v, want ErrTransportFrameTooLarge", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("overflow handler did not terminate promptly")
	}
	if verifier.called {
		t.Fatal("overflow reached Broker.Execute/verifier")
	}
	_ = client.Close()
	<-writeDone
}

func TestHandleConnectionSlowLorisDiesAtDeadline(t *testing.T) {
	verifier := &captureTransportVerifier{contract: brokerTestContract()}
	b := newTransportTestBroker(t, verifier, map[string]any{"active": false})

	server, client := net.Pipe()
	shortServer := &shortDeadlineConn{Conn: server, window: 50 * time.Millisecond}
	done := make(chan error, 1)
	go func() { done <- b.HandleConnection(shortServer) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("slow loris unexpectedly succeeded")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("slow loris connection was not killed by deadline")
	}
	if verifier.called {
		t.Fatal("slow loris reached Broker.Execute/verifier")
	}
	_ = client.Close()
}

func TestHandleConnectionMalformedJSONNeverInvokesBrokerExecute(t *testing.T) {
	verifier := &captureTransportVerifier{contract: brokerTestContract()}
	b := newTransportTestBroker(t, verifier, map[string]any{"active": false})

	server, client := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- b.HandleConnection(server) }()

	if _, err := client.Write([]byte(`{"type":`)); err != nil {
		t.Fatal(err)
	}
	_ = client.Close()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("malformed JSON unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("malformed JSON handler hung")
	}
	if verifier.called {
		t.Fatal("malformed JSON reached Broker.Execute/verifier")
	}
}

func TestHandleConnectionClosesConnectionOnReject(t *testing.T) {
	verifier := &captureTransportVerifier{contract: brokerTestContract()}
	b := newTransportTestBroker(t, verifier, map[string]any{"active": false})

	server, client := net.Pipe()
	done := make(chan error, 1)
	go func() { done <- b.HandleConnection(server) }()
	if _, err := client.Write([]byte("not-json")); err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
	<-done
}
