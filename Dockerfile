FROM golang:1.26 AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o wordle .

FROM scratch
WORKDIR /app
COPY --from=builder /app/wordle .
COPY static/ ./static/
EXPOSE 8080
CMD ["./wordle"]
