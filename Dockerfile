# Stage 1: Build the Go application
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    freetype-dev \
    harfbuzz \
    ca-certificates \
    ttf-freefont \
    && rm -rf /var/cache/apk/*

ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROME_PATH=/usr/bin/chromium-browser

WORKDIR /app

# Copy go.mod and go.sum for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source files
COPY cmd cmd
COPY internal internal

# Debug what we have
RUN echo "Listing /app:" && ls -la && echo "Listing cmd:" && ls -la cmd/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -v -o fastfill ./cmd/server

# Stage 2: Create the final, minimal image
FROM alpine:latest

WORKDIR /root/

# Copy the compiled binary from the builder stage
COPY --from=builder /app/fastfill .

# Install necessary runtime dependencies (e.g., CA certificates for HTTPS)
RUN apk --no-cache add ca-certificates tzdata

# Expose the port the app runs on
EXPOSE 8080

# Define the command to run the application
CMD ["./fastfill"]