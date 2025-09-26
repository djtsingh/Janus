# Step 1: Build the application
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /janus ./cmd/janus

# Step 2: Create the final, minimal image
FROM alpine:latest
WORKDIR /
COPY --from=builder /janus .
COPY web ./web
COPY config.dev.json .

# Expose the port our app will run on
EXPOSE 8080

# The command to run when the container starts
CMD ["/janus", "-config", "config.dev.json"]