# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
# If you add dependencies later, uncomment:
# COPY go.sum ./
# RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# Runtime stage
FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]