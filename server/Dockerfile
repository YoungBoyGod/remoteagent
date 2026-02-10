FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
USER appuser
COPY --from=builder /out/server /usr/local/bin/server
EXPOSE 40001
ENTRYPOINT ["/usr/local/bin/server"]
