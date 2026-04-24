FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download && go mod verify

COPY . .

RUN go build -o d2-mcp .

FROM alpine:3.23

# rsvg-convert rasterizes D2 SVGs to PNG. Fontconfig/DejaVu provide
# fallback fonts when diagrams reference system fonts in addition to D2's
# embedded WOFF fonts.
RUN apk add --no-cache \
    fontconfig \
    librsvg \
    ttf-dejavu

WORKDIR /app

COPY --from=builder /app/d2-mcp .

# Set working directory to /data for file operations
WORKDIR /data

EXPOSE 8080
ENTRYPOINT ["/app/d2-mcp"]
