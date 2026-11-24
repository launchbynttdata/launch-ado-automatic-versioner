FROM golang:1.27.4@sha256:5e8afd493a7124a8586e134d9b532731161d4b9becf9dee5a5956b107426f93d AS builder

ARG BINARY_NAME=aav

WORKDIR /src

COPY . .

RUN go build -o /${BINARY_NAME} ./cmd/aav/

FROM ubuntu:24.04@sha256:2198e4c65076e9cb0b3d695219df667c0e3b0072da64ca3c2648aca17dcecaf5 AS final

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
