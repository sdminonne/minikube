# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building and copying source
RUN go mod download

# Copy the source code
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY third_party/ third_party/

# Build the provider
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o minikube-capi-provider cmd/minikube-capi-provider/main.go

# Final stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/minikube-capi-provider .

USER 65532:65532

ENTRYPOINT ["/minikube-capi-provider"]
