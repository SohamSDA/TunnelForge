# TunnelForge

TunnelForge is a learning-focused HTTP tunneling project inspired by tools such as ngrok. Its goal is to expose a web application running on a developer's machine through a temporary public URL.

For example, a local application running at:

```text
http://localhost:3000
```

will eventually be reachable through an address similar to:

```text
https://abc123.tunnelforge.dev
```

TunnelForge is being built from the ground up to learn Go, networking, HTTP, WebSockets, concurrency, protocol design, testing, and production service architecture.

## Project rewrite

TunnelForge originally started as a TypeScript monorepo with:

- Node.js and TypeScript
- Express for the relay HTTP server
- `ws` for WebSocket communication
- An npm workspace containing the CLI, relay server, and shared protocol package
- An in-memory tunnel and pending-request registry

The TypeScript prototype established the main tunnel flow, but it remained an early proof of concept. It had incomplete protocol definitions, incorrect HTTP-handler wiring, inconsistent body serialization, missing tests, and no production deployment or security layer.

The project is now being rewritten in Go. The rewrite is intentionally being developed in small, tested phases so that every language and system-design concept is understood before the next layer is introduced.

Go was selected because it provides:

- Standalone binaries for distributing the CLI
- A strong standard HTTP library
- Goroutines and channels for concurrent tunnel requests
- Straightforward cross-platform compilation
- Explicit error handling and resource ownership
- Race detection and strong tooling for concurrent network services

The original TypeScript implementation remains available in Git history.

## How TunnelForge will work

TunnelForge will contain two executable programs:

1. **Relay server** — runs on a publicly reachable server and accepts public HTTP traffic.
2. **TunnelForge CLI** — runs on the developer's computer and forwards requests to a local application.

The CLI creates an outgoing, long-lived WebSocket connection to the relay. This works even when the developer's machine is behind a router, NAT, or firewall because the connection is initiated from inside the network.

```text
Internet visitor
       |
       | Public HTTP request
       v
TunnelForge relay server
       |
       | Long-lived WebSocket connection
       v
TunnelForge CLI
       |
       | Local HTTP request
       v
Application on localhost:3000
```

The initial request flow will be:

1. The CLI connects to the relay through WebSocket.
2. The CLI asks the relay to register a tunnel.
3. The relay generates a temporary subdomain and associates it with that connection.
4. A visitor sends an HTTP request to the tunnel's public URL.
5. The relay extracts the subdomain and finds the connected CLI.
6. The relay assigns the request a unique ID and sends it through the WebSocket.
7. The CLI recreates the request against the developer's local application.
8. The CLI sends the local application's response back with the same request ID.
9. The relay matches the response to the waiting public request and returns it to the visitor.

Unique request IDs allow many HTTP requests to travel concurrently through the same tunnel without mixing their responses.

## Planned Go structure

The project will grow gradually into a structure similar to:

```text
TunnelForge/
|-- cmd/
|   |-- relay/
|   |   `-- main.go
|   `-- tunnelforge/
|       `-- main.go
|-- internal/
|   |-- protocol/
|   |-- relay/
|   `-- client/
|-- integration/
|-- go.mod
|-- go.sum
`-- README.md
```

- `cmd/relay` is the entry point for the relay-server executable.
- `cmd/tunnelforge` is the entry point for the user-facing CLI.
- `internal/protocol` defines the messages shared by the relay and CLI.
- `internal/relay` will contain tunnel registration, routing, and request correlation.
- `internal/client` will contain the WebSocket client and localhost forwarding behavior.
- `integration` will test the complete request path.


