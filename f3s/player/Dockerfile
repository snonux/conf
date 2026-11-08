# syntax=docker/dockerfile:1

# ---- Builder stage ----
FROM docker.io/library/golang:1.26-alpine@sha256:87c65a688975c0b1d4585d204874c45ca78354670e3a5f11368c5a492303d696 AS builder

WORKDIR /build

# Install git for go mod download if needed.
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the binary.
RUN GOMAXPROCS=2 CGO_ENABLED=0 GOOS=linux go build -p 2 -ldflags="-s -w" 	-o player ./cmd/player

# ---- Runtime stage ----
FROM docker.io/library/alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

# Install ffmpeg and ffprobe.
RUN apk add --no-cache ffmpeg

# Create a non-root user (nobody already exists as 65534).
USER 65534:65534

WORKDIR /app

# Copy the binary and static assets from the builder.
COPY --from=builder --chown=65534:65534 /build/player /app/player
COPY --from=builder --chown=65534:65534 /build/web /app/web

# Expose the default HTTP port.
EXPOSE 8080

ENTRYPOINT ["/app/player"]
