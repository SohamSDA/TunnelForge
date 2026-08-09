package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SohamSDA/TunnelForge/internal/client"
	"github.com/SohamSDA/TunnelForge/internal/protocol"
	"github.com/SohamSDA/TunnelForge/internal/relay"
)

func TestTunnelRoundTrip(t *testing.T) {
	localApplication := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		w.Header().Set("X-Local-App", "true")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, "%s %s?%s %s %s", request.Method, request.URL.Path, request.URL.RawQuery, request.Header.Get("X-Test"), body)
	}))
	defer localApplication.Close()

	tunnelServer, err := relay.NewServer("http://tunnelforge.test", "secret", log.New(io.Discard, "", 0), relay.WithPathRouting())
	if err != nil {
		t.Fatal(err)
	}
	defer tunnelServer.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/tunnels", tunnelServer.TunnelHandler)
	mux.HandleFunc("/", tunnelServer.PublicHandler)
	publicRelay := httptest.NewServer(mux)
	defer publicRelay.Close()

	localURL, _ := url.Parse(localApplication.URL)
	localHost, localPortText, _ := net.SplitHostPort(localURL.Host)
	localPort, _ := strconv.ParseUint(localPortText, 10, 16)
	relayURL, _ := url.Parse("ws" + strings.TrimPrefix(publicRelay.URL, "http") + "/tunnels")
	ready := make(chan protocol.Registered, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clientDone := make(chan error, 1)
	go func() {
		clientDone <- client.Run(ctx, client.Config{
			LocalHost: localHost, LocalPort: uint16(localPort), RelayURL: relayURL, Token: "secret",
			Logger: log.New(io.Discard, "", 0), OnReady: func(tunnel protocol.Registered) { ready <- tunnel },
		})
	}()

	var registration protocol.Registered
	select {
	case registration = <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("CLI did not register a tunnel")
	}
	registeredURL, _ := url.Parse(registration.URL)

	request, _ := http.NewRequest(http.MethodPost, publicRelay.URL+registeredURL.Path+"/webhook?source=test", bytes.NewBufferString("payload"))
	request.Header.Set("X-Test", "forwarded")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("public request failed: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, http.StatusCreated, body)
	}
	if response.Header.Get("X-Local-App") != "true" {
		t.Fatalf("local response header was not forwarded: %v", response.Header)
	}
	if got, want := string(body), "POST /webhook?source=test forwarded payload"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestTunnelHandlesConcurrentRequests(t *testing.T) {
	localApplication := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		delay, _ := strconv.Atoi(request.URL.Query().Get("delay"))
		time.Sleep(time.Duration(delay) * time.Millisecond)
		fmt.Fprint(w, request.URL.Query().Get("id"))
	}))
	defer localApplication.Close()

	tunnelServer, _ := relay.NewServer("http://tunnelforge.test", "", log.New(io.Discard, "", 0))
	defer tunnelServer.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/tunnels", tunnelServer.TunnelHandler)
	mux.HandleFunc("/", tunnelServer.PublicHandler)
	publicRelay := httptest.NewServer(mux)
	defer publicRelay.Close()

	localURL, _ := url.Parse(localApplication.URL)
	host, portText, _ := net.SplitHostPort(localURL.Host)
	port, _ := strconv.ParseUint(portText, 10, 16)
	websocketURL, _ := url.Parse("ws" + strings.TrimPrefix(publicRelay.URL, "http") + "/tunnels")
	ready := make(chan protocol.Registered, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = client.Run(ctx, client.Config{LocalHost: host, LocalPort: uint16(port), RelayURL: websocketURL, Logger: log.New(io.Discard, "", 0), OnReady: func(value protocol.Registered) { ready <- value }})
	}()
	registration := <-ready
	subdomain := strings.Split(strings.TrimPrefix(registration.URL, "http://"), ".")[0]

	const count = 20
	var waitGroup sync.WaitGroup
	waitGroup.Add(count)
	for index := range count {
		go func() {
			defer waitGroup.Done()
			request, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/work?id=%d&delay=%d", publicRelay.URL, index, count-index), nil)
			request.Header.Set("X-Tunnel-Subdomain", subdomain)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Errorf("request %d failed: %v", index, err)
				return
			}
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if string(body) != strconv.Itoa(index) {
				t.Errorf("request %d got response %q", index, body)
			}
		}()
	}
	waitGroup.Wait()
}
