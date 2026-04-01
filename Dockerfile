## Multi-stage build: compile the Go binary, then create a slim runtime image
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG BUILD_DATE=""
ARG VCS_REF=""
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /app/janus ./cmd/janus

FROM debian:bookworm-slim
LABEL org.opencontainers.image.created=$BUILD_DATE
LABEL org.opencontainers.image.revision=$VCS_REF
RUN apt-get update && apt-get install -y openssl ca-certificates curl && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /app/janus /app/janus
COPY entrypoint.sh /app/entrypoint.sh
COPY assets/ /app/assets/
RUN chmod +x /app/entrypoint.sh
RUN groupadd -r janus && useradd -r -g janus -d /app -s /sbin/nologin janus || true
RUN chown -R janus:janus /app
USER janus
EXPOSE 8080 8081
HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
    CMD curl -k -f https://localhost:8080/health || exit 1
ENTRYPOINT ["/app/entrypoint.sh"]
