# Use the official Go image as the base image
FROM golang:1.24 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Install Templ using Go install
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy the source code into the container
COPY . .

# Generate Go code from Templ files
RUN templ generate

# Build the Go application with static linking
RUN CGO_ENABLED=0 GOOS=linux go build -o app .

# Use a lightweight Alpine image for the final stage
FROM alpine:latest

# Install certificates for HTTPS support
RUN apk --no-cache add ca-certificates

# Set the working directory in the final image
WORKDIR /app

# Copy the built application from the builder stage
COPY --from=builder /app/app .

# Expose the application port (adjust as needed)
EXPOSE 8080

# Run the application
CMD ["./app"]