package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SohamSDA/TunnelForge/internal/relay"
)

type healthResponse struct {
	Status string `json:"status"`
}

func healthHandler(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)
	publicURL := env("TUNNELFORGE_PUBLIC_URL", "http://localhost:8080")
	var options []relay.Option
	if env("TUNNELFORGE_ROUTING_MODE", "subdomain") == "path" {
		options = append(options, relay.WithPathRouting())
	}
	tunnelServer, err := relay.NewServer(publicURL, os.Getenv("TUNNELFORGE_TOKEN"), logger, options...)
	if err != nil {
		logger.Fatalf("configure relay: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", healthHandler)
	mux.HandleFunc("/tunnels", tunnelServer.TunnelHandler)
	mux.HandleFunc("/", tunnelServer.PublicHandler)

	server := &http.Server{
		Addr:              ":" + env("PORT", "8080"),
		Handler:           requestLog(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		tunnelServer.Close()
		_ = server.Shutdown(shutdownContext)
	}()

	logger.Printf("relay starting address=%s public_url=%s", server.Addr, publicURL)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatalf("relay failed: %v", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func requestLog(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, request)
		logger.Printf("request method=%s host=%s path=%s duration=%s", request.Method, request.Host, request.URL.Path, time.Since(started).Round(time.Millisecond))
	})
}
