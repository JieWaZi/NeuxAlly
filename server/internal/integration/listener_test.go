package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIntegrationListenerLifecycle(t *testing.T) {
	root := t.TempDir()
	reservation, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reservation.Addr().(*net.TCPAddr).Port
	_ = reservation.Close()
	data, _ := json.Marshal(map[string]any{"ip": "127.0.0.1", "port": port, "enabled": true})
	if err := os.WriteFile(filepath.Join(root, "integration-api.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	listener := NewListener(root, func(string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	})
	if err := listener.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	old := listener.GetAPIStatus()
	if !old.Available || old.Port != port {
		t.Fatal(old)
	}
	blocked, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer blocked.Close()
	if err := listener.SetAPIAddress(context.Background(), "127.0.0.1", blocked.Addr().(*net.TCPAddr).Port); err == nil {
		t.Fatal("port conflict accepted")
	}
	if current := listener.GetAPIStatus(); current != old {
		t.Fatal("failed switch lost working listener", current)
	}
	if err := listener.Configure(context.Background(), old.IP, old.Port, false, ""); err != nil {
		t.Fatal(err)
	}
	if listener.GetAPIStatus().Available || listener.Settings().Enabled {
		t.Fatal("disabled API remains available")
	}
	if err := listener.Configure(context.Background(), old.IP, old.Port, true, ""); err != nil {
		t.Fatal(err)
	}
	if !listener.GetAPIStatus().Available {
		t.Fatal("re-enable failed")
	}
	listener.Close()
	listener.Close()
}

func TestIntegrationDisableClosesListenerBeforeServeStarts(t *testing.T) {
	bound, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer bound.Close()
	listener := NewListener(t.TempDir(), func(string) http.Handler { return http.NotFoundHandler() })
	// 模拟已经登记监听套接字，但 Serve goroutine 尚未被调度的真实窗口。
	listener.ctx, listener.cancel = context.WithCancel(context.Background())
	listener.api = ListenerStatus{IP: "127.0.0.1", Port: bound.Addr().(*net.TCPAddr).Port, Available: true}
	listener.server = &http.Server{Handler: http.NotFoundHandler()}
	listener.listener = bound
	defer listener.Close()

	if err := listener.Configure(context.Background(), listener.api.IP, listener.api.Port, false, ""); err != nil {
		t.Fatal(err)
	}
	if err := bound.SetDeadline(time.Now()); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("disabled listener retained bound socket before Serve: %v", err)
	}
	if listener.GetAPIStatus().Available || listener.Settings().Enabled {
		t.Fatal("disabled API remains available")
	}
}
