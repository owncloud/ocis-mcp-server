# Stage 1: Build
FROM golang:1.27-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/ocis-mcp-server ./cmd/ocis-mcp-server

# Stage 2: Runtime
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/ocis-mcp-server /usr/local/bin/ocis-mcp-server

# Run as non-root
RUN addgroup -S mcp && adduser -S mcp -G mcp
USER mcp

EXPOSE 8090

ENTRYPOINT ["ocis-mcp-server"]
