# Build stage
FROM golang:alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o dbmate-deployer ./cmd/dbmate-deployer

# Runtime stage
FROM alpine:3.21

# Install required packages
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /build/dbmate-deployer /usr/local/bin/dbmate-deployer

WORKDIR /tmp

ENTRYPOINT ["/usr/local/bin/dbmate-deployer"]
