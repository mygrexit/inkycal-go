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
	return serve(ctx, &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}, ln)
}

// shutdownGrace is how long running requests (a save, a preview) get after Ctrl+C.
var shutdownGrace = 5 * time.Second

// serve runs hs until ctx ends, then lets running requests finish within shutdownGrace
// and cuts off what is still running after that.
func serve(ctx context.Context, hs *http.Server, ln net.Listener) error {
	shutdown := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		err := hs.Shutdown(shut)
		if errors.Is(err, context.DeadlineExceeded) {
			err = hs.Close()
		}
		shutdown <- err
	}()
	if err := hs.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return <-shutdown
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
