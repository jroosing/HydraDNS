# =============================================================================
# HydraDNS Dockerfile (Go) - Security Hardened
# =============================================================================

FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY zones/ ./zones/
COPY docker/ ./docker/
COPY hydradns.example.yaml ./

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/hydradns ./cmd/hydradns
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dnsquery ./cmd/dnsquery

FROM alpine:3.21 AS runtime

ARG UID=1000
ARG GID=1000

RUN addgroup -g ${GID} hydradns && \
    adduser -D -u ${UID} -G hydradns -h /app -s /sbin/nologin hydradns && \
    apk add --no-cache ca-certificates && \
    rm -rf /var/cache/apk/* /tmp/*

ENV HYDRADNS_CONFIG=/app/config/hydradns.yaml \
    LOG_LEVEL=INFO

WORKDIR /app

COPY --from=builder /out/hydradns /app/hydradns
COPY --from=builder /out/dnsquery /app/dnsquery

COPY --chown=hydradns:hydradns zones/ /app/zones/
RUN mkdir -p /app/config && chown hydradns:hydradns /app/config
COPY --chown=hydradns:hydradns docker/hydradns.yaml /app/config/hydradns.yaml

RUN chmod 755 /app/hydradns /app/dnsquery && \
    chmod 644 /app/config/hydradns.yaml && \
    chmod 644 /app/zones/*.zone 2>/dev/null || true

EXPOSE 1053/udp
EXPOSE 1053/tcp

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/app/dnsquery", "--server", "127.0.0.1:1053", "--name", "example.com", "--qtype", "1", "--quiet"]

USER hydradns

CMD ["/app/hydradns"]
