# Verification record

Verified on 29 October 2026 in the local macOS development environment.

## Passed

- `make verify`: Go formatting, vet, unit/integration tests, race detection and builds.
- `make binaries VERSION=1.0.0`: generated `bin/tunnelforge` and `bin/relay`.
- `./bin/tunnelforge --version`: returned `tunnelforge 1.0.0`.
- `docker compose config --quiet`: Compose configuration is valid.
- `go run ./cmd/demo`: started all three components and printed a usable tunnel URL.
- Manual curl through that tunnel: health, POST JSON, query parameters,
  forwarded metadata, request ID, and the exact binary bytes `00 01 02 7f 80 fe ff`.

Integration tests exercise actual HTTP and WebSocket connections. They cover
round trips, concurrent request matching, bad token rejection, redirect
preservation, repeated Set-Cookie headers, binary round trips, hop-by-hop header
filtering, request/response limits, local application failure, timeout propagation
to the local application's context, and reconnection. The relay unit tests
include rejection of a response from the wrong tunnel.

## External verification still required

The installed OrbStack runtime was started to attempt `docker build -t
tunnelforge:college .`. The image build was blocked while retrieving base-image
metadata: Docker Hub presented a certificate for `blocked.anexgate.com`, not
`registry-1.docker.io`. No TLS checks were disabled. Retry the build from a network
that can access Docker Hub normally. Container execution and container health
have therefore not been verified here.

No hosting service or public hostname has been selected. A public relay has not
been deployed. Follow DEPLOYMENT.md, supply the host and shared secret, then test
HTTPS/WSS access from a separate network. The college implementation and local
verification are complete; public-host and container verification remain external
deployment steps.
