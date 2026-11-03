// Run with: go run ./research/experiment -out output/research/data
// The workload is a closed-loop POST echo test, not an Internet benchmark.
package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SohamSDA/TunnelForge/internal/client"
	"github.com/SohamSDA/TunnelForge/internal/protocol"
	"github.com/SohamSDA/TunnelForge/internal/relay"
)

type Result struct {
	Mode        string  `json:"mode"`
	Bytes       int     `json:"payload_bytes"`
	Concurrency int     `json:"concurrency"`
	Repeat      int     `json:"repeat"`
	Requests    int     `json:"requests"`
	Errors      int     `json:"errors"`
	Duration    float64 `json:"duration_seconds"`
	RPS         float64 `json:"requests_per_second"`
	Median      float64 `json:"median_ms"`
	P95         float64 `json:"p95_ms"`
}

func main() {
	out := flag.String("out", "output/research/data", "results directory")
	n := flag.Int("n", 256, "requests per measured run")
	flag.Parse()
	if *n < 32 {
		log.Fatal("n must be at least 32")
	}
	if err := run(*out, *n); err != nil {
		log.Fatal(err)
	}
}

func run(out string, n int) error {
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	raw, err := os.Create(filepath.Join(out, "latencies.csv"))
	if err != nil {
		return err
	}
	defer raw.Close()
	csvOut := csv.NewWriter(raw)
	defer csvOut.Flush()
	csvOut.Write([]string{"mode", "payload_bytes", "concurrency", "repeat", "request_index", "latency_ms", "ok"})
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", 400)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(body)
	}))
	defer app.Close()
	s, err := relay.NewServer("http://relay.test", "experiment", log.New(io.Discard, "", 0), relay.WithPathRouting())
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/tunnels", s.TunnelHandler)
	mux.HandleFunc("/", s.PublicHandler)
	public := httptest.NewServer(mux)
	defer public.Close()
	defer s.Close()
	local, _ := url.Parse(app.URL)
	host, pt, _ := net.SplitHostPort(local.Host)
	port, _ := strconv.Atoi(pt)
	ws, _ := url.Parse("ws" + strings.TrimPrefix(public.URL, "http") + "/tunnels")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan protocol.Registered, 1)
	go client.Run(ctx, client.Config{LocalHost: host, LocalPort: uint16(port), RelayURL: ws, Token: "experiment", Logger: log.New(io.Discard, "", 0), OnReady: func(r protocol.Registered) {
		select {
		case ready <- r:
		case <-ctx.Done():
		}
	}})
	var tunnelURL string
	select {
	case r := <-ready:
		u, _ := url.Parse(r.URL)
		tunnelURL = public.URL + u.Path + "/echo"
	case <-time.After(5 * time.Second):
		return fmt.Errorf("registration timeout")
	}
	var results []Result
	for _, size := range []int{1024, 65536} {
		payload := bytes.Repeat([]byte{0, 1, 127, 128, 254, 255, 34, 10}, size/8)
		for _, concurrency := range []int{1, 8, 32} {
			for repeat := 1; repeat <= 3; repeat++ {
				// Alternate mode order to reduce a fixed-order advantage.
				modes := []string{"direct", "tunnel"}
				if repeat%2 == 0 {
					modes = []string{"tunnel", "direct"}
				}
				for _, mode := range modes {
					endpoint := app.URL + "/echo"
					if mode == "tunnel" {
						endpoint = tunnelURL
					}
					transport := http.DefaultTransport.(*http.Transport).Clone()
					transport.Proxy = nil
					transport.MaxIdleConnsPerHost = 64
					transport.MaxConnsPerHost = 64
					transport.DisableCompression = true
					hc := &http.Client{Transport: transport, Timeout: 10 * time.Second}
					send := func() bool {
						resp, e := hc.Post(endpoint, "application/octet-stream", bytes.NewReader(payload))
						if e != nil {
							return false
						}
						body, e := io.ReadAll(resp.Body)
						resp.Body.Close()
						return e == nil && resp.StatusCode == 200 && bytes.Equal(body, payload)
					}
					for range 32 {
						if !send() {
							return fmt.Errorf("warmup failure %s", mode)
						}
					}
					samples := make([]float64, n)
					success := make([]bool, n)
					jobs := make(chan int, n)
					for i := 0; i < n; i++ {
						jobs <- i
					}
					close(jobs)
					var wg sync.WaitGroup
					start := time.Now()
					for range concurrency {
						wg.Add(1)
						go func() {
							defer wg.Done()
							for i := range jobs {
								t := time.Now()
								success[i] = send()
								samples[i] = float64(time.Since(t).Nanoseconds()) / 1e6
							}
						}()
					}
					wg.Wait()
					elapsed := time.Since(start).Seconds()
					failed := 0
					for i, ms := range samples {
						if !success[i] {
							failed++
						}
						csvOut.Write([]string{mode, strconv.Itoa(size), strconv.Itoa(concurrency), strconv.Itoa(repeat), strconv.Itoa(i), fmt.Sprintf("%.6f", ms), strconv.FormatBool(success[i])})
					}
					ordered := append([]float64(nil), samples...)
					sort.Float64s(ordered)
					r := Result{Mode: mode, Bytes: size, Concurrency: concurrency, Repeat: repeat, Requests: n, Errors: failed, Duration: elapsed, RPS: float64(n) / elapsed, Median: ordered[(n-1)/2], P95: ordered[(95*n+99)/100-1]}
					results = append(results, r)
					transport.CloseIdleConnections()
					fmt.Printf("%s bytes=%d c=%d repeat=%d p50=%.3fms p95=%.3fms rps=%.1f errors=%d\n", mode, size, concurrency, repeat, r.Median, r.P95, r.RPS, r.Errors)
				}
			}
		}
	}
	csvOut.Flush()
	if err := csvOut.Error(); err != nil {
		return err
	}
	resultFile, err := os.Create(filepath.Join(out, "results.json"))
	if err != nil {
		return err
	}
	defer resultFile.Close()
	encoder := json.NewEncoder(resultFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]any{"timestamp_utc": time.Now().UTC().Format(time.RFC3339), "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "logical_cpus": runtime.NumCPU(), "gomaxprocs": runtime.GOMAXPROCS(0), "requests_per_run": n, "warmup_requests_per_run": 32, "repeats": 3, "results": results})
}
