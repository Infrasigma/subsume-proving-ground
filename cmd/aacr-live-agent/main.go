package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: aacr-live-agent /run/aacr/broker.sock")
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
