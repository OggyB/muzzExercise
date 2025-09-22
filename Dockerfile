# Stage 1: Build
FROM golang:1.25.1 AS builder

WORKDIR /app

# Install deps
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN go build -o bin/muzz-exercise ./cmd/server

# Stage 2 - Runtime
FROM gcr.io/distroless/base-debian12 AS runtime

WORKDIR /app

# Copy binary only
COPY --from=builder /app/bin/muzz-exercise .

# Non-root user (security best practice)
USER nonroot:nonroot

CMD ["./muzz-exercise"]