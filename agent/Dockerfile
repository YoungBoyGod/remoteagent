FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /out/agent ./cmd/agent

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10002 appuser
USER appuser
COPY --from=builder /out/agent /usr/local/bin/agent
EXPOSE 40002
ENTRYPOINT ["/usr/local/bin/agent"]
