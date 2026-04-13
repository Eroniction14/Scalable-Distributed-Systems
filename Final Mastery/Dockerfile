# ─── Stage 1: Build ────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy dependency files first — Docker caches this layer.
# If go.mod/go.sum don't change, dependencies aren't re-downloaded on every build.
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o album-store .

# ─── Stage 2: Run ──────────────────────────────────────────────────────────
FROM alpine:3.20

# CA certificates needed for HTTPS calls to AWS services (DynamoDB, S3, SQS)
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/album-store .

EXPOSE 8080

CMD ["./album-store"]