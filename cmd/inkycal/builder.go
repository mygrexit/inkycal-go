package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"inkycal/internal/app"
	"inkycal/internal/builder"
	"inkycal/internal/config"
)

// runBuilder serves the visual editor until Ctrl+C.
func runBuilder(settingsPath, stateDir, listen string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return serveBuilder(ctx, settingsPath, stateDir, listen, os.Stdout)
}

// builderPaths fills in the defaults: without --settings the builder edits the
// settings.json the panel would use, else ./settings.json (created on the first save);
// its state lives in the user cache directory.
func builderPaths(settingsPath, stateDir string) (string, string) {
	if settingsPath == "" {
		if p, err := config.FindSettings(""); err == nil {
			settingsPath = p
		} else {
			settingsPath = "settings.json"
		}
	}
	if abs, err := filepath.Abs(settingsPath); err == nil {
		settingsPath = abs
	}
	if stateDir == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			dir = os.TempDir()
		}
		stateDir = filepath.Join(dir, "inkycal-builder")
	}
	return settingsPath, stateDir
}

// serveBuilder serves until ctx ends and prints the addresses to open on out.
func serveBuilder(ctx context.Context, settingsPath, stateDir, listen string, out io.Writer) error {
	settingsPath, stateDir = builderPaths(settingsPath, stateDir)
	srv, err := builder.New(builder.Config{
		SettingsPath: settingsPath, StateDir: stateDir, Registry: app.Registry, Catalog: app.Catalog,
	})
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	addr := ln.Addr().(*net.TCPAddr)
	fmt.Fprintf(out, "inkycal builder — editing %s\n", settingsPath)
	for _, host := range hosts(addr) {
		fmt.Fprintf(out, "  open http://%s/#token=%s\n", net.JoinHostPort(host, fmt.Sprint(addr.Port)), srv.Token())
	}
	fmt.Fprintln(out, "Ctrl+C to stop.")
	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(shut)
	}()
	if err := hs.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// hosts lists the addresses to print: the bound one, or every IPv4 address when
// listening on all interfaces.
func hosts(addr *net.TCPAddr) []string {
	if !addr.IP.IsUnspecified() {
		return []string{addr.IP.String()}
	}
	var out []string
	ifaces, _ := net.InterfaceAddrs()
	for _, a := range ifaces {
		if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil {
			out = append(out, ipn.IP.String())
		}
	}
	if len(out) == 0 {
		out = []string{"localhost"}
	}
	return out
}
