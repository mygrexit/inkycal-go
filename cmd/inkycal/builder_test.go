package main

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inkycal/internal/config"
)

// Without flags the builder edits the panel's settings.json when there is one, else
// ./settings.json, and keeps its state in the user cache directory.
func TestBuilderPaths(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	saved := config.SearchPaths
	t.Cleanup(func() { config.SearchPaths = saved })
	config.SearchPaths = []string{filepath.Join(dir, "boot", "settings.json")}
	t.Setenv("INKYCAL_SETTINGS_PATH", "")
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))

	sp, state := builderPaths("", "")
	if want := filepath.Join(dir, "settings.json"); sp != want {
		t.Errorf("no settings anywhere: %s, want %s", sp, want)
	}
	if want := filepath.Join(dir, "cache", "inkycal-builder"); state != want {
		t.Errorf("state dir %s, want %s", state, want)
	}

	panel := filepath.Join(dir, "panel.json")
	if err := os.WriteFile(panel, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("INKYCAL_SETTINGS_PATH", panel)
	if sp, _ := builderPaths("", ""); sp != panel {
		t.Errorf("panel settings: %s, want %s", sp, panel)
	}

	sp, state = builderPaths("rel.json", "/var/tmp/x")
	if sp != filepath.Join(dir, "rel.json") || state != "/var/tmp/x" {
		t.Errorf("explicit: %s %s", sp, state)
	}
}

// The builder prints a working address with its token, rejects requests without the
// token and stops cleanly when the context ends.
func TestServeBuilder(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- serveBuilder(ctx, filepath.Join(dir, "settings.json"), filepath.Join(dir, "state"), "127.0.0.1:0", pw)
		pw.Close()
	}()

	var addr, token string
	sc := bufio.NewScanner(pr)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if u, ok := strings.CutPrefix(line, "open http://"); ok {
			hostPort, frag, _ := strings.Cut(u, "/#token=")
			addr, token = hostPort, frag
		}
		if strings.HasPrefix(line, "Ctrl+C") {
			break
		}
	}
	go io.Copy(io.Discard, pr)
	if addr == "" || token == "" {
		t.Fatalf("no address with token printed (addr %q)", addr)
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		t.Fatalf("printed address %q: %v", addr, err)
	}

	get := func(tok string) int {
		req, _ := http.NewRequest("GET", "http://"+addr+"/api/catalog", nil)
		if tok != "" {
			req.Header.Set("X-Inkycal-Token", tok)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if code := get(token); code != http.StatusOK {
		t.Errorf("with token: %d", code)
	}
	if code := get(""); code != http.StatusUnauthorized {
		t.Errorf("without token: %d, want 401", code)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("stop: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("builder did not stop")
	}
}

func TestHosts(t *testing.T) {
	if got := hosts(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}); len(got) != 1 || got[0] != "127.0.0.1" {
		t.Errorf("bound address: %v", got)
	}
	if got := hosts(&net.TCPAddr{IP: net.IPv4zero, Port: 8080}); len(got) == 0 {
		t.Error("all interfaces: no address to print")
	}
}
