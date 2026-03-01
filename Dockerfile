# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags '-w -s' \
    -o bin/promsketch-dropin ./cmd/promsketch-dropin

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/bin/promsketch-dropin .

# Create config directory
RUN mkdir -p /etc/promsketch

# Expose port
EXPOSE 9100

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9100/health || exit 1

# Run as non-root user
RUN adduser -D -u 1000 promsketch
USER promsketch

ENTRYPOINT ["./promsketch-dropin"]
CMD ["--config.file=/etc/promsketch/config.yaml"]
