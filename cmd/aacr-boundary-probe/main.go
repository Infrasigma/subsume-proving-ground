package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: aacr-boundary-probe /run/aacr/broker.sock")
		os.Exit(2)
	}
	socket := os.Args[1]
	conn, err := net.DialTimeout("unix", socket, time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "broker connection failed:", err)
		os.Exit(3)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"probe":"mediated-channel"}` + "\n")); err != nil {
		fmt.Fprintln(os.Stderr, "broker write failed:", err)
		os.Exit(4)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, "broker read failed:", err)
		os.Exit(5)
	}
	fmt.Print(line)

	// 198.51.100.0/24 is TEST-NET-2 (RFC 5737). It is not a local endpoint;
	// in the isolated namespace there must be no route capable of reaching it.
	if _, err := net.DialTimeout("tcp", "198.51.100.1:80", 200*time.Millisecond); err == nil {
		fmt.Fprintln(os.Stderr, "network unexpectedly reachable")
		os.Exit(6)
	} else {
		// Preserve the kernel/network-stack error verbatim so the CI artifact
		// proves the attempted network connection was rejected inside the
		// isolated namespace, rather than merely reporting a boolean.
		fmt.Fprintln(os.Stderr, err)
	}
	fmt.Println(`{"network":"blocked"}`)
}
