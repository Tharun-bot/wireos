# Stage 1: Build
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o wireos .

# Stage 2: scratch — zero attack surface
FROM scratch

# Wire client makes outbound HTTPS calls — needs CA bundle
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/wireos /wireos

EXPOSE 8081

ENTRYPOINT ["/wireos"]