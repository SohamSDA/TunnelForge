FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/relay ./cmd/relay

FROM alpine:3.22

RUN addgroup -S tunnelforge && adduser -S -G tunnelforge tunnelforge
COPY --from=build /out/relay /usr/local/bin/relay
USER tunnelforge
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:${PORT:-8080}/health || exit 1
ENTRYPOINT ["/usr/local/bin/relay"]
