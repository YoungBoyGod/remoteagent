FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/agent ./cmd/agent

FROM alpine:3.20
RUN adduser -D -u 10002 appuser
USER appuser
COPY --from=builder /out/agent /usr/local/bin/agent
EXPOSE 40002
ENTRYPOINT ["/usr/local/bin/agent"]
