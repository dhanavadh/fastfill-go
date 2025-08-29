FROM golang:1.23-alpine AS builder

# Install Chrome dependencies and Chrome
RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    freetype-dev \
    harfbuzz \
    ca-certificates \
    ttf-freefont \
    && rm -rf /var/cache/apk/*

# Set environment variable for Chrome
ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROME_PATH=/usr/bin/chromium-browser

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code (explicitly include cmd and internal)
COPY cmd/ cmd/
COPY internal/ internal/
COPY *.go ./

# Debug: List what was copied
RUN echo "Contents of /app:" && ls -la /app && echo "Contents of /app/cmd:" && ls -la /app/cmd/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM alpine:latest

# Install Chrome and dependencies for runtime
RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    freetype-dev \
    harfbuzz \
    ca-certificates \
    ttf-freefont \
    && rm -rf /var/cache/apk/*

# Set environment variable for Chrome
ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROME_PATH=/usr/bin/chromium-browser

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/server .

# Create directories for config and static files
RUN mkdir -p config static

EXPOSE 8080

CMD ["./server"]