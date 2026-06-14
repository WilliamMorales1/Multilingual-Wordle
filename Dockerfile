FROM golang:1.26 AS builder
WORKDIR /app
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o wordle .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY favicon.ico /favicon.ico
COPY frontend/ /frontend/
WORKDIR /app
COPY --from=builder /app/wordle .
EXPOSE 8080
CMD ["./wordle"]
