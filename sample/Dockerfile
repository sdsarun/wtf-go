# 1. Build stage
FROM golang:1.22-alpine AS builder

# Why: Alpine = small image, Go installed
WORKDIR /app

# Copy go mod first (for caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN go build -o app

# 2. Run stage (small, secure)
FROM alpine:3.19

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/app .

# Expose port
EXPOSE 4000

# Run app
CMD ["./app"]