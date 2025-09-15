# Stage 1: Build the application
FROM golang:1.24-alpine AS builder

# Set the necessary environment variables for cross-compilation
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -o webssh .

# Stage 2: Create the final, minimal image
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/webssh .
COPY --from=builder /app/static ./static
EXPOSE 8080 8443
CMD ["./webssh"]