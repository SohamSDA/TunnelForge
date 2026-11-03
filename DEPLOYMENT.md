# Deployment

TunnelForge can run on any container host that supports long-lived WebSocket connections. Path routing works with the default hostname supplied by a hosting provider, so a custom domain is not required for the first deployment.

## Required configuration

Set these environment variables on the relay service:

```text
PORT=8080
TUNNELFORGE_PUBLIC_URL=https://your-service-hostname.example
TUNNELFORGE_ROUTING_MODE=path
TUNNELFORGE_TOKEN=<long-random-secret>
```

Generate a token locally:

```bash
openssl rand -hex 32
```

The platform may set `PORT` automatically. Use that value instead of overriding it when required by the platform.

## Container deployment

Build and run locally:

```bash
docker compose up --build
```

Run the CLI against it:

```bash
TUNNELFORGE_TOKEN=local-development-token \
go run ./cmd/tunnelforge --relay ws://localhost:8080/tunnels 3000
```

For a public container host:

1. Create a web service from this Git repository.
2. Select Dockerfile deployment.
3. Configure the environment variables above.
4. Deploy and verify `https://<service>/health`.
5. Run the CLI with `wss://<service>/tunnels` and the same token.

## Wildcard subdomain routing

For ngrok-style URLs, set:

```text
TUNNELFORGE_ROUTING_MODE=subdomain
TUNNELFORGE_PUBLIC_URL=https://tunnelforge.example.com
```

Then configure wildcard DNS and TLS for `*.tunnelforge.example.com` to point to the relay. Path routing is recommended for the first deployment because it works without wildcard DNS.

## Public deployment checklist

Use one replica: connections and tunnel ownership live in a single process.
Enable long-lived WebSocket upgrades on the platform's HTTP proxy, with an idle
timeout greater than 60 seconds. TLS should terminate at the platform proxy;
public callers use HTTPS and the CLI uses WSS. Keep the token in the platform's
secret settings. The relay executable refuses to start with an empty token or an
unknown routing mode.

The Docker image includes a health check. Set a platform health check to `/health`
and expose the platform-assigned `PORT`. The declared public URL must be an
HTTP/HTTPS origin without credentials, query, fragment or a path prefix.

## Verify the deployed service end to end

On your computer, terminal 1:

```bash
go run ./cmd/demoapp
```

Terminal 2 (replace the example hostname, then supply the actual secret):

```bash
read -s 'TUNNELFORGE_TOKEN?Relay token: '; echo
export TUNNELFORGE_TOKEN
go run ./cmd/tunnelforge --host 127.0.0.1 \
  --relay wss://YOUR-SERVICE-HOSTNAME/tunnels 3000
```

The `read` command above is for macOS zsh. Other shells can set the environment
variable through their standard secret input method.

Terminal 3, using the printed HTTPS tunnel URL:

```bash
TUNNEL_URL='https://YOUR-SERVICE-HOSTNAME/t/ACTUAL-ID'
curl -f "$TUNNEL_URL/api/status"
curl -f -X POST -H 'Content-Type: application/json' \
  -d '{"source":"public-demo"}' "$TUNNEL_URL/api/echo"
curl -s "$TUNNEL_URL/api/binary" | od -An -tx1
```

Open `$TUNNEL_URL/` on a phone using mobile data. Successful access from that
separate network proves Internet reachability. Never describe a localhost-only
demonstration as a public deployment. After relay/CLI restarts, use the newly
printed tunnel URL. Do not place secrets in source control.

## Deployment status

Deployment files are supplied; publishing requires your hosting account and
selected service. Record the actual service URL and test date after deployment.
No public deployment is asserted by this repository.
