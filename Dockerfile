# =============================================================================
# HydraDNS Dockerfile (Go) - Security Hardened
# =============================================================================

FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY docker/ ./docker/

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/hydradns ./cmd/hydradns

FROM alpine:3.21 AS runtime

ARG UID=1000
ARG GID=1000

RUN addgroup -g ${GID} hydradns && \
    adduser -D -u ${UID} -G hydradns -h /app -s /sbin/nologin hydradns && \
    apk add --no-cache ca-certificates && \
    rm -rf /var/cache/apk/* /tmp/*

ENV HYDRADNS_CONFIG=/app/config/hydradns.yaml \
    HYDRADNS_LOGGING_LEVEL=INFO

WORKDIR /app

COPY --from=builder /out/hydradns /app/hydradns

RUN mkdir -p /app/config && chown hydradns:hydradns /app/config
COPY --chown=hydradns:hydradns docker/hydradns.yaml /app/config/hydradns.yaml

RUN chmod 755 /app/hydradns && \
    chmod 644 /app/config/hydradns.yaml

EXPOSE 1053/udp
EXPOSE 1053/tcp

USER hydradns

CMD ["/app/hydradns"]
