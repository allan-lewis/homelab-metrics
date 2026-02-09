set shell := ["bash", "-euo", "pipefail", "-c"]

# Run with live reload (dev port = 9203)
dev:
    air

# Run once (dev port)
run:
    PORT=9203 go run .

# Build local binary
build:
    go build

# Run all tests
test:
    go test ./...

# Clean and normalize go.mod/go.sum
tidy:
    go mod tidy

# Remove build and temporary files
clean:
    rm -rf tmp homelab-metrics
