package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "network-probe" {
		probeNetwork(os.Args[2])
		return
	}
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: aacr-live-agent /run/aacr/broker.sock | network-probe host:port")
		os.Exit(2)
	}
	rawB64 := os.Getenv("AACR_ENVELOPE_B64")
	if rawB64 == "" {
		fmt.Fprintln(os.Stderr, "AACR_ENVELOPE_B64 is required")
		os.Exit(2)
	}
	raw, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		fmt.Fprintln(os.Stderr, "decode envelope:", err)
		os.Exit(2)
	}
	conn, err := net.DialTimeout("unix", os.Args[1], time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "broker connection failed:", err)
		os.Exit(3)
	}
	defer conn.Close()
	if _, err := conn.Write(raw); err != nil {
		fmt.Fprintln(os.Stderr, "broker write failed:", err)
		os.Exit(4)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	response, err := io.ReadAll(conn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "broker read failed:", err)
		os.Exit(5)
	}
	if len(response) == 0 {
		fmt.Fprintln(os.Stderr, "empty broker response")
		os.Exit(5)
	}
	_, _ = os.Stdout.Write(response)
}

func probeNetwork(target string) {
	conn, err := net.DialTimeout("tcp", target, 500*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		fmt.Fprintln(os.Stderr, "network unexpectedly reachable")
		os.Exit(6)
	}
	fmt.Fprintln(os.Stdout, err)
	// The live-fire harness asserts the precise kernel error. Keep the probe
	// itself successful for any unreachable destination so the test can inspect
	// whether the namespace produced ECONNREFUSED or ENETUNREACH.
	if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "network is unreachable") {
		return
	}
	os.Exit(7)
}
