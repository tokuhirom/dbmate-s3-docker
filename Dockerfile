# Build stage
FROM golang:alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o dbmate-s3-docker ./cmd/dbmate-s3-docker

# Runtime stage
FROM alpine:3.21

# Install required packages
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /build/dbmate-s3-docker /usr/local/bin/dbmate-s3-docker

WORKDIR /tmp

ENTRYPOINT ["/usr/local/bin/dbmate-s3-docker"]
