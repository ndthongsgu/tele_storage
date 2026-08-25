# Build Stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates tzdata

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build lightweight binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o tele_storage main.go

# Production Stage
FROM alpine:3.20

WORKDIR /app

# Install CA certificates for HTTPS requests to Telegram API
RUN apk add --no-cache ca-certificates tzdata

# Copy binary and static files from builder
COPY --from=builder /app/tele_storage .
COPY --from=builder /app/static ./static

# Expose port
EXPOSE 8080

# Run service
ENTRYPOINT ["./tele_storage"]
