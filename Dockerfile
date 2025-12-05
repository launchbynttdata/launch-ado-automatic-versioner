FROM golang:1.25 AS builder

ARG BINARY_NAME=aav

WORKDIR /src

COPY . .

RUN go build -o /${BINARY_NAME} ./cmd/aav/

FROM ubuntu:24.04 AS final

ARG BINARY_NAME=aav
# Install ca-certificates for HTTPS requests
RUN apt-get update && \
    apt-get install -y ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    update-ca-certificates && \
    groupadd -g 1001 appgroup && \
    useradd -u 1001 -g appgroup -s /bin/false appuser

WORKDIR /app

# Copy binary and documentation from builder stage
COPY --from=builder --chown=appuser:appgroup /${BINARY_NAME} /app/${BINARY_NAME}

# Switch to non-root user
USER appuser

# Run the application
CMD ["./${BINARY_NAME}"]
