# Build stage
FROM golang:1.23.3-alpine AS builder

WORKDIR /workspace

# Copy Go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY main.go .

# Build
RUN CGO_ENABLED=0 go build -o kubesnooze

# Run stage
FROM alpine:3.19

WORKDIR /
COPY --from=builder /workspace/kubesnooze .

ENTRYPOINT ["/kubesnooze"]
