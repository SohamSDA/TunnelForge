package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/SohamSDA/TunnelForge/internal/client"
	"github.com/SohamSDA/TunnelForge/internal/protocol"
)

const version = "dev"

type config struct {
	localHost string
	localPort uint16
	relayURL  *url.URL
	token     string
}

func parseConfig(args []string, stderr io.Writer) (config, bool, error) {
	flags := flag.NewFlagSet("tunnelforge", flag.ContinueOnError)
	flags.SetOutput(stderr)

	localHost := flags.String("host", "localhost", "local application hostname")
	relayAddress := flags.String("relay", "ws://localhost:8080/tunnels", "TunnelForge relay WebSocket URL")
	showVersion := flags.Bool("version", false, "print the TunnelForge version")
	token := flags.String("token", os.Getenv("TUNNELFORGE_TOKEN"), "relay authentication token (or TUNNELFORGE_TOKEN)")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "Usage: tunnelforge [options] <port>\n\nOptions:\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return config{}, false, err
	}
	if *showVersion {
		return config{}, true, nil
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return config{}, false, errors.New("exactly one local port is required")
	}

	port, err := strconv.ParseUint(flags.Arg(0), 10, 16)
	if err != nil || port == 0 {
		return config{}, false, fmt.Errorf("invalid port %q: must be between 1 and 65535", flags.Arg(0))
	}
	if net.ParseIP(*localHost) == nil && !validHostname(*localHost) {
		return config{}, false, fmt.Errorf("invalid local hostname %q", *localHost)
	}

	relayURL, err := url.ParseRequestURI(*relayAddress)
	if err != nil || (relayURL.Scheme != "ws" && relayURL.Scheme != "wss") || relayURL.Host == "" {
		return config{}, false, fmt.Errorf("invalid relay URL %q: expected ws:// or wss:// URL", *relayAddress)
	}

	return config{localHost: *localHost, localPort: uint16(port), relayURL: relayURL, token: *token}, false, nil
}

func validHostname(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	for _, label := range splitHost(host) {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
				(character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

func splitHost(host string) []string {
	var labels []string
	start := 0
	for index, character := range host {
		if character == '.' {
			labels = append(labels, host[start:index])
			start = index + 1
		}
	}
	return append(labels, host[start:])
}

type connector func(context.Context, client.Config) error

func run(ctx context.Context, args []string, stdout, stderr io.Writer, connect connector) int {
	configuration, showVersion, err := parseConfig(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "tunnelforge: %v\n", err)
		return 2
	}
	if showVersion {
		fmt.Fprintf(stdout, "tunnelforge %s\n", version)
		return 0
	}

	localAddress := net.JoinHostPort(configuration.localHost, strconv.Itoa(int(configuration.localPort)))
	fmt.Fprintf(stdout, "Local application: http://%s\n", localAddress)
	fmt.Fprintf(stdout, "Relay: %s\n", configuration.relayURL)
	err = connect(ctx, client.Config{
		LocalHost: configuration.localHost, LocalPort: configuration.localPort, RelayURL: configuration.relayURL, Token: configuration.token,
		OnReady: func(tunnel protocol.Registered) {
			fmt.Fprintf(stdout, "Forwarding: %s -> http://%s\n", tunnel.URL, localAddress)
		},
	})
	if err != nil {
		fmt.Fprintf(stderr, "tunnelforge: %v\n", err)
		return 1
	}
	return 0
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr, client.Run))
}
