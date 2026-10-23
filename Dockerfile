FROM golang:1.23-alpine AS builder

WORKDIR /workspace

# Copy go mod and sum
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY main.go client.go ./

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o webhook .

# Minimal runtime image
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/webhook .
USER 65532:65532

ENTRYPOINT ["/webhook"]
