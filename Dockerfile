FROM golang:1.26 AS builder
WORKDIR /app
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o wordgo ./cmd/server

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY frontend/public/ /frontend/public/
WORKDIR /app
COPY --from=builder /app/wordgo .
EXPOSE 8080
CMD ["./wordgo"]
