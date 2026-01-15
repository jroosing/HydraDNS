# =============================================================================
# HydraDNS Dockerfile (Go) - Security Hardened
# =============================================================================

FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/hydradns ./cmd/hydradns

FROM alpine:3.21 AS runtime

ARG UID=1000
ARG GID=1000

RUN addgroup -g ${GID} hydradns && \
    adduser -D -u ${UID} -G hydradns -h /app -s /sbin/nologin hydradns && \
    apk add --no-cache ca-certificates && \
    rm -rf /var/cache/apk/* /tmp/*

ENV HYDRADNS_DB=/data/hydradns.db \
    HYDRADNS_LOGGING_LEVEL=INFO

WORKDIR /app

COPY --from=builder /out/hydradns /app/hydradns

RUN mkdir -p /data && chown hydradns:hydradns /data

RUN chmod 755 /app/hydradns

EXPOSE 1053/udp
EXPOSE 1053/tcp
EXPOSE 8080/tcp

VOLUME ["/data"]

USER hydradns

CMD ["/app/hydradns", "--db", "/data/hydradns.db"]
