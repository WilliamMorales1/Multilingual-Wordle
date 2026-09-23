FROM golang:1.27 AS builder
WORKDIR /app
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o wordgo ./cmd/server

FROM node:20 AS frontend-builder
WORKDIR /frontend
COPY frontend/ .
RUN npm ci && npm run build

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=frontend-builder /frontend/public/ /frontend/public/
WORKDIR /app
COPY --from=builder /app/wordgo .
EXPOSE 8080
CMD ["./wordgo"]
