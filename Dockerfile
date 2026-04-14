# Stage 1: Build
FROM golang:1.26-alpine@sha256:c2a1f7b2095d046ae14b286b18413a05bb82c9bca9b25fe7ff5efef0f0826166 AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/ocis-mcp-server ./cmd/ocis-mcp-server

# Stage 2: Runtime
FROM alpine:3.23@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/ocis-mcp-server /usr/local/bin/ocis-mcp-server

# Run as non-root
RUN addgroup -S mcp && adduser -S mcp -G mcp
USER mcp

EXPOSE 8090

ENTRYPOINT ["ocis-mcp-server"]
