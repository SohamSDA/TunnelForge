# TunnelForge demonstration

From the repository directory:

```bash
go run ./cmd/demo
```

Copy the printed tunnel base URL, for example:

```bash
TUNNEL_URL='http://127.0.0.1:EXAMPLE_PORT/t/EXAMPLE_ID'
```

Replace that example with the actual printed URL. The port and ID change on restart.

## Demonstrate the working features

```bash
curl -i "$TUNNEL_URL/"
curl -i "$TUNNEL_URL/api/status"
curl -i -X POST -H 'Content-Type: application/json' -H 'X-Demo: college' \
  -d '{"name":"Soham","project":"TunnelForge"}' "$TUNNEL_URL/api/echo?source=presentation"
curl -i -X PUT -d 'updated value' "$TUNNEL_URL/api/echo"
curl -s "$TUNNEL_URL/api/binary" | od -An -tx1
curl -i "$TUNNEL_URL/missing"
seq 1 10 | xargs -I{} -P10 curl -s "$TUNNEL_URL/api/echo?id={}"
curl -i "$TUNNEL_URL/api/delay?seconds=31"
```

Expected results: HTML/JSON responses; the POST method, query, headers and body
echoed accurately; binary bytes `00 01 02 7f 80 fe ff`; a forwarded 404 for an
unknown route; ten independent concurrent responses; a 504 after 30 seconds.
Each successful forwarded response includes an `X-Request-Id` header.

## Explain the implementation

"TunnelForge uses an outbound WebSocket from a client to a relay. The relay
registers a temporary tunnel and assigns each public request a unique ID. The
client forwards each request to the local application and returns the response
with the same ID. This supports concurrent requests without mixing responses.
JSON carries the protocol and Base64 carries binary bodies. The relay enforces
authentication, request limits and timeouts. Cancellation stops local requests
when the public request ends. Heartbeats and ping/pong detect disconnected peers,
and the CLI retries broken connections with bounded exponential backoff."

Show `internal/protocol/message.go`, `internal/relay/server.go`,
`internal/client/client.go` and the integration tests.

## Automated evidence

```bash
go test -v ./integration
go test -race ./...
```

These cover authentication rejection, reconnect, a real POST round trip,
concurrency, cookies, redirects, binary bodies, hop-by-hop header filtering,
limits, timeout cancellation and an unavailable local application.

## Authentication and reconnection as separate processes

Use the relay and CLI commands in README.md. A wrong token is rejected without
retrying. Stop and restart the relay to show the CLI reconnecting. Reconnection
creates a **new** URL; copy it before making more requests. Stopping the local
application yields a 502. Stopping the CLI removes its tunnel (404).

## Presentation boundaries

The one-command demo runs on this computer. Demonstrate actual Internet access
only after deploying the relay with HTTPS/WSS and testing it from another device.
The project buffers HTTP bodies and does not support streaming, WebSocket
upgrades through a tunnel, TCP tunnelling or permanent URLs.
