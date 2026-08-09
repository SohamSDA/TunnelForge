# TunnelForge

TunnelForge is a reverse HTTP tunnel written in Go. It forwards requests from a relay to a localhost application through a persistent WebSocket-connected CLI.

> The tunnel works locally but is not publicly deployed.

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
