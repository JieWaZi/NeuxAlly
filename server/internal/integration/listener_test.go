package integration

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
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
