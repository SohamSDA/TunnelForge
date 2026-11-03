// Package demo provides a local application for demonstrating the tunnel.
package demo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>TunnelForge</title><style>body{font:18px system-ui;background:#10243e;color:white;max-width:780px;margin:80px auto;padding:24px}h1{font-size:52px;color:#60d5e2}a{color:#60d5e2}code{background:#203958;padding:4px}li{margin:18px 0}</style><h1>TunnelForge works.</h1><p>This page was served by your local application through the relay and WebSocket tunnel.</p><ul><li><a href="api/status">Check API status</a></li><li><a href="api/echo?source=browser">Inspect a request</a></li><li><a href="api/binary">Download binary data</a></li></ul><p>POST JSON to <code>/api/echo</code> to see its method, headers and body.</p></html>`)
	})
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "project": "TunnelForge", "time": time.Now().UTC()})
	})
	mux.HandleFunc("/api/echo", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"method": r.Method, "path": r.URL.Path, "query": r.URL.Query(), "headers": r.Header, "body": string(body)})
	})
	mux.HandleFunc("/api/binary", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0, 1, 2, 127, 128, 254, 255})
	})
	mux.HandleFunc("/api/delay", func(w http.ResponseWriter, r *http.Request) {
		seconds, err := strconv.Atoi(r.URL.Query().Get("seconds"))
		if err != nil || seconds < 0 || seconds > 35 {
			http.Error(w, "seconds must be 0-35", 400)
			return
		}
		timer := time.NewTimer(time.Duration(seconds) * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			fmt.Fprint(w, "delayed response")
		case <-r.Context().Done():
		}
	})
	return mux
}
