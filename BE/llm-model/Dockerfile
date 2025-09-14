# ======================
# Builder stage (compile Go)
# ======================
FROM golang:1.24.1 AS builder

# Set working directory
WORKDIR /app

# Copy Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy prompts
COPY prompts /app/prompts

# Copy the source code
COPY . .

# Build the Go binary statically (no GLIBC issues)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o llm-model .

# ======================
# Final stage (minimal runtime)
# ======================
FROM alpine:latest

# Set working directory
WORKDIR /root/

# Add /usr/local/bin to PATH
ENV PATH="/usr/local/bin:${PATH}"

# Copy the statically compiled binary from builder
COPY --from=builder /app/llm-model .

# Expose the port your API runs on
EXPOSE 3030

# Default command
CMD ["./llm-model"]
