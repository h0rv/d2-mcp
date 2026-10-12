FROM golang:1.27.2-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /d2-mcp .

FROM alpine:3.23
RUN addgroup -S d2 && adduser -S -G d2 d2 && mkdir -p /data && chown d2:d2 /data
COPY --from=builder /d2-mcp /usr/local/bin/d2-mcp
USER d2
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/d2-mcp"]
