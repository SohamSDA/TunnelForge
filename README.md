# TunnelForge

TunnelForge is a reverse HTTP tunnel written in Go. It forwards requests from a relay to a localhost application through a persistent WebSocket-connected CLI.

The college-project implementation includes HTTP tunnelling, authentication,
concurrent request matching, heartbeat/ping handling, automatic reconnection,
request cancellation and Docker packaging. Public hosting requires a deployment
on your own server or container platform; a live public instance is not bundled.

## Quickest start

Install the Go toolchain required by `go.mod`, then run:

```bash
go mod download
go run ./cmd/demo
```

The demo starts a local application, a relay and a tunnel client on available
loopback ports. Open the printed tunnel URL **including the trailing slash**.
It prints a ready-to-copy POST command as well. Press Ctrl+C to stop everything.
This is a local demonstration; a localhost URL is accessible only on this machine.

The demo application has these routes:

| Route | Demonstrates |
| --- | --- |
| `/` | HTML served through the tunnel |
| `/api/status` | JSON response |
| `/api/echo?source=demo` | GET/POST/PUT/PATCH/DELETE, query, headers and body |
| `/api/binary` | Arbitrary bytes transported without corruption |
| `/api/delay?seconds=31` | Relay timeout (504) after 30 seconds |

The Python server below is an alternative static demo. It supports GET and HEAD;
use `cmd/demo` for POST and other API demonstrations.

## Verification and binaries

```bash
make verify
make binaries VERSION=1.0.0
./bin/tunnelforge --version
```

`make verify` formats the code, runs vet, unit/integration tests, race detection,
and builds all packages. Integration tests start local HTTP/WebSocket listeners.
Use `go test -v ./integration` for the end-to-end scenarios.

See [DEMO.md](DEMO.md) for the presentation walkthrough,
[ARCHITECTURE.md](ARCHITECTURE.md) for diagrams and limitations, and
[DEPLOYMENT.md](DEPLOYMENT.md) for public hosting.

## Local setup

### 1. Start your application

Run an HTTP application locally and verify it. Example for port `3000`:

```bash
curl -i http://127.0.0.1:3000
```

For a quick demo:

```bash
mkdir -p /tmp/tunnelforge-demo
printf '<h1>TunnelForge works!</h1>\n' > /tmp/tunnelforge-demo/index.html
python3 -m http.server 3000 --directory /tmp/tunnelforge-demo
```

### 2. Start the relay

```bash
TUNNELFORGE_PUBLIC_URL=http://localhost:8080 \
TUNNELFORGE_ROUTING_MODE=path \
TUNNELFORGE_TOKEN=local-secret \
go run ./cmd/relay
```

Check it:

```bash
curl http://localhost:8080/health
```

Expected:

```json
{"status":"ok"}
```

### 3. Start the CLI

```bash
TUNNELFORGE_TOKEN=local-secret \
go run ./cmd/tunnelforge \
  --host 127.0.0.1 \
  --relay ws://localhost:8080/tunnels \
  3000
```

The CLI prints a temporary URL:

```text
Forwarding: http://localhost:8080/t/<tunnel-id> -> http://127.0.0.1:3000
```

### 4. Test the tunnel

```bash
curl -i http://localhost:8080/t/<tunnel-id>
```

Test a local API route:

```bash
curl -i http://localhost:8080/t/<tunnel-id>/api/users
```

Test a JSON request:

```bash
curl -i -X POST \
  -H 'Content-Type: application/json' \
  -d '{"name":"Soham"}' \
  http://localhost:8080/t/<tunnel-id>/api/users
```
