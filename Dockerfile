# Build stage
# We use a separate build stage so the final image doesn't carry
# the Go toolchain, just the compiled binary. This keeps the
# production image small and reduces the attack surface.
FROM golang:1.25-alpine AS builder
# Install git, needed for go mod download to fetch from GitHub.
RUN apk add --no-cache git

WORKDIR /app

# Copy dependency files first and download before copying source.
# Docker layer caches this step, if go.mod and go.sum haven't changed
# the module download is skipped on rebuild. This makes iterative
# builds significantly faster.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build the binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o payfake ./cmd/api

#  Run stage
# Alpine is minimal, ~5MB base image. We only copy the compiled
# binary from the builder stage, nothing else.
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/payfake .

EXPOSE 8080

CMD ["./payfake"]
