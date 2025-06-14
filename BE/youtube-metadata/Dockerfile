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
RUN CGO_ENABLED=0 GOOS=linux go build -o my-youtube-metadata-app .

# Use a minimal alpine image for the final stage.
FROM alpine:latest

# Set the working directory.
WORKDIR /root/

# Copy the binary from the builder stage.
COPY --from=builder /app/my-youtube-metadata-app .

# Expose the port the app runs on.
EXPOSE 6060

# Command to run the application.
CMD ["./my-youtube-metadata-app"]