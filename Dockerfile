FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download && go mod verify

COPY . .

RUN go build -o d2-mcp .

FROM alpine:3.22

RUN apk add --no-cache imagemagick librsvg

WORKDIR /app

COPY --from=builder /app/d2-mcp .

# Set working directory to /data for file operations
WORKDIR /data

EXPOSE 8080
ENTRYPOINT ["/app/d2-mcp"]
