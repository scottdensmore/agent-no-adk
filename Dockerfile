# Build stage
FROM golang:1.25-bookworm AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /bin/sre-agent ./cmd/server

# Production runtime stage
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=builder /bin/sre-agent /app/sre-agent
COPY sample_logs/ /app/sample_logs/

USER nonroot:nonroot
EXPOSE 8080
ENV PORT=8080

ENTRYPOINT ["/app/sre-agent"]
