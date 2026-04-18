# Stage 1: Build
FROM golang:1.26-alpine@sha256:c2a1f7b2095d046ae14b286b18413a05bb82c9bca9b25fe7ff5efef0f0826166 AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/ocis-mcp-server ./cmd/ocis-mcp-server

# Stage 2: Runtime
FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/ocis-mcp-server /usr/local/bin/ocis-mcp-server

# Run as non-root
RUN addgroup -S mcp && adduser -S mcp -G mcp
USER mcp

EXPOSE 8090

ENTRYPOINT ["ocis-mcp-server"]
