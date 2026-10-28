FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG SERVICE=cp
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -trimpath -o /build/server ./cmd/${SERVICE}

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/server ./server

EXPOSE 8080 8081

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- "http://localhost:${PORT:-8080}/health" || exit 1

CMD ["./server"]
