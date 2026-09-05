package boundary

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// ListenUnix creates a private filesystem Unix socket for the mediated channel.
// The socket is created with mode 0600 so an unrelated host user cannot inject
// requests merely by discovering the path.
func ListenUnix(path string) (net.Listener, error) {
	if path == "" || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("socket path must be absolute")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create socket directory: %w", err)
	}
	_ = os.Remove(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on unix socket: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		_ = l.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("chmod unix socket: %w", err)
	}
	return l, nil
}
