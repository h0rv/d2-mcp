FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download && go mod verify

COPY . .

RUN go build -o d2-mcp .

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /app/d2-mcp .

EXPOSE 8080
CMD ["./d2-mcp", "--sse", "--port", "8080"]