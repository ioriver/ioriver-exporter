# Build stage
FROM golang:1.24-trixie AS builder

# Install build dependencies
# RUN apk add --no-cache git ca-certificates
# RUN apk add --no-cache shadow
RUN useradd -r ioriver-exporter

WORKDIR /build

COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

RUN CGO_ENABLED=0 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o ioriver-exporter \
    ./cmd/ioriver-exporter


# Runtime stage
FROM scratch

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /build/ioriver-exporter /ioriver-exporter

USER ioriver-exporter

EXPOSE 8080

ENTRYPOINT ["/ioriver-exporter", "-listen=0.0.0.0:8080"]
