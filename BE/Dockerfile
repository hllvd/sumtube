# Use the official Golang image to create a build artifact.
FROM golang:1.24.1 as builder

# Set the working directory inside the container.
WORKDIR /app

# Copy the Go module files and download dependencies.
COPY go.mod .
COPY go.sum .
RUN go mod download

# Copy the source code into the container.
COPY . .

# Build the Go application.
RUN CGO_ENABLED=0 GOOS=linux go build -o my-go-app .

# Use a minimal alpine image for the final stage.
FROM alpine:latest

# Set the working directory.
WORKDIR /root/

# Install dependencies for yt-dlp (if needed)
RUN apk add --no-cache ffmpeg wget python3

# Download and install yt-dlp (standalone binary)
RUN wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -O /usr/local/bin/yt-dlp && \
    chmod a+rx /usr/local/bin/yt-dlp

# Add /usr/local/bin to the PATH
ENV PATH="/usr/local/bin:${PATH}"

# Verify yt-dlp installation
RUN yt-dlp --version

# Copy the binary from the builder stage.
COPY --from=builder /app/my-go-app .

# Expose the port the app runs on.
EXPOSE 8080

# Command to run the application.
CMD ["./my-go-app"]