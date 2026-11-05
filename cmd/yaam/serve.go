package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wk-j/yaam/internal/app"
	"github.com/wk-j/yaam/internal/config"
	"github.com/wk-j/yaam/internal/keychain"
)

const defaultServeAddr = "127.0.0.1:7341"

// serveInfo is serve.json, read by `yaam open`.
type serveInfo struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
}

func cmdServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	addr := flags.String("addr", defaultServeAddr, "")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("usage: yaam serve [-addr HOST:PORT]")
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return err
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("refusing to listen on %s: the web UI binds to loopback addresses only", host)
	}
	p, err := homePaths()
	if err != nil {
		return err
	}
	if err := config.EnsureHome(p.home); err != nil {
		return err
	}
	relayLn, err := listenSocket(p.socket)
	if err != nil {
		return err
	}
	defer relayLn.Close()

	a, err := app.New(p.home, keychain.Security{})
	if err != nil {
		return err
	}
	defer a.Close()

	explicit := false
	flags.Visit(func(f *flag.Flag) { explicit = explicit || f.Name == "addr" })
	ln, err := listenServe(*addr, !explicit)
	if err != nil {
		return err
	}
	raw := make([]byte, 32)
	rand.Read(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)
	port := ln.Addr().(*net.TCPAddr).Port
	hosts := []string{ln.Addr().String(), fmt.Sprintf("localhost:%d", port)}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	srv := &http.Server{
		Handler:           a.Handler(token, hosts),
		ReadHeaderTimeout: 10 * time.Second,
		// Event streams end when yaam stops, so Shutdown need not wait.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	go func() {
		<-ctx.Done()
		relayLn.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()
	go func() {
		if err := a.ServeRelay(relayLn); err != nil {
			fmt.Fprintln(os.Stderr, "yaam: relay:", err)
			stop()
		}
	}()

	url := "http://" + ln.Addr().String()
	info, _ := json.Marshal(serveInfo{URL: url, Token: token, PID: os.Getpid()})
	if err := config.WriteFileAtomic(p.serveJSON, info); err != nil {
		return err
	}
	defer os.Remove(p.serveJSON)
	fmt.Fprintf(os.Stderr, "yaam: web UI on %s (open it with `yaam open`)\nyaam: relay socket %s\n", url, p.socket)
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// listenServe listens on addr. When addr is the default and another process
// already holds it, it listens on a free loopback port instead; an address
// given with -addr is never swapped.
func listenServe(addr string, fallback bool) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if fallback && errors.Is(err, syscall.EADDRINUSE) {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	return ln, err
}

// listenSocket listens on the relay socket. A socket that still accepts
// connections means another yaam serve runs; a stale one is removed.
func listenSocket(path string) (net.Listener, error) {
	if _, err := os.Lstat(path); err == nil {
		if c, err := net.DialTimeout("unix", path, time.Second); err == nil {
			c.Close()
			return nil, errors.New("yaam serve is already running")
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}
