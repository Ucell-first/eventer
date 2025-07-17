FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application from cmd directory
RUN CGO_ENABLED=0 GOOS=linux go build -o log-processor ./cmd

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/log-processor .

# Copy .env file
COPY .env .

# Expose ports
EXPOSE 8080 8081

# Run the application
CMD ["./log-processor"]
