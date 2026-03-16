FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./cmd/server

FROM alpine:3.20

# Install runtime deps including docker CLI and dockerize for socket wait
RUN apk add --no-cache \
    ca-certificates \
    git \
    docker-cli \
    curl \
    bash

# Create docker group with GID 999 (matches host Docker socket GID on most systems)
# and add a non-root app user to it so the socket is accessible
RUN addgroup -g 999 docker 2>/dev/null || true && \
    adduser -D -u 1000 -G docker appuser 2>/dev/null || true

WORKDIR /app
COPY --from=builder /app/server /app/server

# Ensure binary is executable
RUN chmod +x /app/server

EXPOSE 8080

# Run as root so Docker socket is always accessible regardless of host GID
# (safest for development; for production scope with group_add in compose)
ENTRYPOINT ["/app/server"]
