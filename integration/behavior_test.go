package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SohamSDA/TunnelForge/internal/client"
	"github.com/SohamSDA/TunnelForge/internal/demo"
	"github.com/SohamSDA/TunnelForge/internal/protocol"
	"github.com/SohamSDA/TunnelForge/internal/relay"
	"github.com/gorilla/websocket"
)

func startRelay(t *testing.T, options ...relay.Option) (*httptest.Server, *url.URL) {
	t.Helper()
	options = append([]relay.Option{relay.WithPathRouting()}, options...)
	s, err := relay.NewServer("http://relay.test", "secret", log.New(io.Discard, "", 0), options...)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/tunnels", s.TunnelHandler)
	mux.HandleFunc("/", s.PublicHandler)
	public := httptest.NewServer(mux)
	t.Cleanup(func() { s.Close(); public.Close() })
	u, _ := url.Parse("ws" + strings.TrimPrefix(public.URL, "http") + "/tunnels")
	return public, u
}

func startClient(t *testing.T, ws *url.URL, local string, token string) (string, context.CancelFunc) {
	t.Helper()
	u, _ := url.Parse(local)
	host, portText, _ := net.SplitHostPort(u.Host)
	port, _ := strconv.Atoi(portText)
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan protocol.Registered, 4)
	done := make(chan error, 1)
	go func() {
		done <- client.Run(ctx, client.Config{LocalHost: host, LocalPort: uint16(port), RelayURL: ws, Token: token, Logger: log.New(io.Discard, "", 0), OnReady: func(r protocol.Registered) {
			select {
			case ready <- r:
			case <-ctx.Done():
			}
		}})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("client did not stop")
		}
	})
	select {
	case r := <-ready:
		u, _ := url.Parse(r.URL)
		return u.Path, cancel
	case err := <-done:
		t.Fatalf("client stopped: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("registration timed out")
	}
	return "", cancel
}

func TestAuthenticationRejectsWithoutRetry(t *testing.T) {
	_, ws := startRelay(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := client.Run(ctx, client.Config{LocalHost: "127.0.0.1", LocalPort: 3000, RelayURL: ws, Token: "wrong", Logger: log.New(io.Discard, "", 0)})
	if !errors.Is(err, client.ErrRejected) {
		t.Fatalf("want rejection, got %v", err)
	}
}

func TestHTTPBehavior(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			w.Header().Set("Location", "https://example.invalid/")
			w.WriteHeader(http.StatusFound)
		case "/cookies":
			w.Header().Add("Set-Cookie", "a=1")
			w.Header().Add("Set-Cookie", "b=2")
		case "/binary":
			w.Header().Set("Content-Type", "application/octet-stream")
			body, _ := io.ReadAll(r.Body)
			w.Write(body)
		case "/headers":
			w.Header().Set("Connection", "X-Private")
			w.Header().Set("X-Private", "hidden")
			w.Write([]byte(r.Header.Get("X-Secret") + "|" + r.Header.Get("X-Forwarded-Host")))
		case "/large":
			w.Write(bytes.Repeat([]byte("x"), protocol.MaxBodyBytes+1))
		default:
			http.NotFound(w, r)
		}
	}))
	defer app.Close()
	public, ws := startRelay(t)
	path, _ := startClient(t, ws, app.URL, "secret")
	hc := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	tests := []struct {
		name, path string
		body       []byte
		status     int
	}{
		{"redirect", "/redirect", nil, 302}, {"cookies", "/cookies", nil, 200},
		{"binary", "/binary", []byte{0, 1, 127, 128, 255}, 200}, {"headers", "/headers", nil, 200},
		{"missing", "/missing", nil, 404}, {"response limit", "/large", nil, 502},
		{"request limit", "/binary", bytes.Repeat([]byte("x"), protocol.MaxBodyBytes+1), 413},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := http.NewRequest("POST", public.URL+path+tt.path, bytes.NewReader(tt.body))
			r.Header.Set("Connection", "X-Secret")
			r.Header.Set("X-Secret", "private")
			resp, err := hc.Do(r)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != tt.status {
				t.Fatalf("status %d body %s", resp.StatusCode, body)
			}
			switch tt.name {
			case "redirect":
				if resp.Header.Get("Location") != "https://example.invalid/" {
					t.Fatal("redirect lost")
				}
			case "cookies":
				if len(resp.Header.Values("Set-Cookie")) != 2 {
					t.Fatal("cookies lost")
				}
			case "binary":
				if !bytes.Equal(body, tt.body) {
					t.Fatalf("binary corrupted: %v", body)
				}
			case "headers":
				if resp.Header.Get("X-Private") != "" || string(body) != "|"+r.URL.Host {
					t.Fatalf("hop headers or forwarding incorrect: %s %v", body, resp.Header)
				}
			}
		})
	}
	app.Close()
	resp, err := hc.Get(public.URL + path + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 502 {
		t.Fatalf("local failure status %d", resp.StatusCode)
	}
}

func TestDemoRoundTrip(t *testing.T) {
	app := httptest.NewServer(demo.Handler())
	defer app.Close()
	public, ws := startRelay(t)
	path, _ := startClient(t, ws, app.URL, "secret")
	resp, err := http.Post(public.URL+path+"/api/echo?source=test", "application/json", strings.NewReader(`{"name":"Soham"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !bytes.Contains(body, []byte(`"method":"POST"`)) || !bytes.Contains(body, []byte(`Soham`)) {
		t.Fatalf("demo response: %s", body)
	}
}

func TestTimeoutCancelsLocalRequest(t *testing.T) {
	cancelled := make(chan struct{}, 1)
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		cancelled <- struct{}{}
	}))
	defer app.Close()
	public, ws := startRelay(t, relay.WithRequestTimeout(150*time.Millisecond))
	path, _ := startClient(t, ws, app.URL, "secret")
	hc := &http.Client{Timeout: 3 * time.Second}
	response, err := hc.Get(public.URL + path + "/slow")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 504 {
		t.Fatalf("want 504, got %d", response.StatusCode)
	}
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("local request was not cancelled")
	}
}

func TestClientReconnectsAfterConnectionCloses(t *testing.T) {
	connections := make(chan struct{}, 4)
	var upgrader = websocket.Upgrader{}
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		message, _ := protocol.NewMessage(protocol.TypeRegistered, "", protocol.Registered{TunnelID: "test", URL: "http://relay.test/t/test"})
		if err := conn.WriteJSON(message); err != nil {
			return
		}
		connections <- struct{}{}
	}))
	defer wsServer.Close()
	u, _ := url.Parse("ws" + strings.TrimPrefix(wsServer.URL, "http"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- client.Run(ctx, client.Config{LocalHost: "127.0.0.1", LocalPort: 3000, RelayURL: u, Logger: log.New(io.Discard, "", 0)})
	}()
	for range 2 {
		select {
		case <-connections:
		case <-time.After(4 * time.Second):
			t.Fatal("client did not reconnect")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("client did not stop")
	}
}
