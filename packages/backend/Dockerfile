FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/cp \
        ./cmd/cp \
    && CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/dp \
        ./cmd/dp

FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app -h /app app

WORKDIR /app
ENV PORT=8080
EXPOSE 8080 8081
USER app
STOPSIGNAL SIGTERM

FROM runtime AS cp

COPY --from=builder /out/cp ./cp

ENV PORT=8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${PORT}/health" >/dev/null || exit 1

CMD ["./cp"]

FROM runtime AS dp

COPY --from=builder /out/dp ./dp

ENV PORT=8081
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${PORT}/health" >/dev/null || exit 1

CMD ["./dp"]

FROM runtime AS combined

ENV CP_PORT=8080 \
    DP_PORT=8081

COPY --from=builder /out/cp ./cp
COPY --from=builder /out/dp ./dp
COPY --chown=app:app docker-entrypoint.sh /app/docker-entrypoint.sh

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${CP_PORT}/health" >/dev/null \
        && wget -qO- "http://127.0.0.1:${DP_PORT}/health" >/dev/null || exit 1

ENTRYPOINT ["/app/docker-entrypoint.sh"]
