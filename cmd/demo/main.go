// The demo command starts a local app, relay and CLI together using free ports.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/SohamSDA/TunnelForge/internal/client"
	"github.com/SohamSDA/TunnelForge/internal/demo"
	"github.com/SohamSDA/TunnelForge/internal/protocol"
	"github.com/SohamSDA/TunnelForge/internal/relay"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	local, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer local.Close()
	public, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer public.Close()
	baseURL := "http://" + public.Addr().String()
	tunnel, err := relay.NewServer(baseURL, "demo-local-only", nil, relay.WithPathRouting())
	if err != nil {
		return err
	}
	defer tunnel.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/tunnels", tunnel.TunnelHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/", tunnel.PublicHandler)
	localServer := &http.Server{Handler: demo.Handler(), ReadHeaderTimeout: 5 * time.Second}
	relayServer := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer localServer.Close()
	defer relayServer.Close()
	errors := make(chan error, 3)
	go func() { errors <- localServer.Serve(local) }()
	go func() { errors <- relayServer.Serve(public) }()
	_, portText, _ := net.SplitHostPort(local.Addr().String())
	port, _ := strconv.Atoi(portText)
	relayURL, _ := url.Parse("ws://" + public.Addr().String() + "/tunnels")
	go func() {
		errors <- client.Run(ctx, client.Config{LocalHost: "127.0.0.1", LocalPort: uint16(port), RelayURL: relayURL, Token: "demo-local-only", OnReady: func(registered protocol.Registered) {
			fmt.Printf("\nLocal application: http://%s\nRelay health: %s/health\n\nOpen this tunnel URL in your browser:\n%s/\n\nTest POST:\ncurl -i -X POST -H 'Content-Type: application/json' -d '{\"name\":\"Soham\"}' '%s/api/echo?source=demo'\n\nPress Ctrl+C to stop all components.\n", local.Addr(), baseURL, registered.URL, registered.URL)
		}})
	}()
	select {
	case <-ctx.Done():
		return nil
	case err := <-errors:
		return err
	}
}
