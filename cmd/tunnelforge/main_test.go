package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/SohamSDA/TunnelForge/internal/client"
	"github.com/SohamSDA/TunnelForge/internal/protocol"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "defaults", args: []string{"3000"}},
		{name: "custom configuration", args: []string{"--host", "127.0.0.1", "--relay", "wss://relay.example.com/tunnels", "8080"}},
		{name: "missing port", wantErr: "exactly one local port"},
		{name: "extra argument", args: []string{"3000", "4000"}, wantErr: "exactly one local port"},
		{name: "zero port", args: []string{"0"}, wantErr: "between 1 and 65535"},
		{name: "large port", args: []string{"65536"}, wantErr: "between 1 and 65535"},
		{name: "non-numeric port", args: []string{"http"}, wantErr: "between 1 and 65535"},
		{name: "invalid host", args: []string{"--host", "bad host", "3000"}, wantErr: "invalid local hostname"},
		{name: "invalid relay scheme", args: []string{"--relay", "https://relay.example.com", "3000"}, wantErr: "expected ws:// or wss://"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configuration, _, err := parseConfig(test.args, &bytes.Buffer{})
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("parseConfig() error = %v, want substring %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseConfig() unexpected error: %v", err)
			}
			if configuration.localPort == 0 || configuration.relayURL == nil {
				t.Fatalf("parseConfig() returned incomplete configuration: %#v", configuration)
			}
		})
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"--version"}, &stdout, &stderr, nil); code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "tunnelforge dev") {
		t.Fatalf("run() output = %q", got)
	}
}

func TestRunValidConfiguration(t *testing.T) {
	var stdout, stderr bytes.Buffer
	connect := func(_ context.Context, configuration client.Config) error {
		configuration.OnReady(protocol.Registered{TunnelID: "test", URL: "https://test.example.com"})
		return nil
	}
	if code := run(context.Background(), []string{"3000"}, &stdout, &stderr, connect); code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "http://localhost:3000") {
		t.Fatalf("run() output = %q", got)
	}
}
