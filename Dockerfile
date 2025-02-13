# Builder stage comune
FROM golang:1.23.2-alpine AS builder

WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

# Build API
RUN CGO_ENABLED=0 GOOS=linux go build -o /gosol-api cmd/api/main.go

# Build Producer
RUN CGO_ENABLED=0 GOOS=linux go build -o /gosol-monitor-producer cmd/monitor/producer/main.go

# Build Consumer
RUN CGO_ENABLED=0 GOOS=linux go build -o /gosol-monitor-consumer cmd/monitor/consumer/main.go

# Build Producer
RUN CGO_ENABLED=0 GOOS=linux go build -o /gosol-txdetails-producer cmd/txdetails/producer/main.go

# Build Consumer
RUN CGO_ENABLED=0 GOOS=linux go build -o /gosol-txdetails-consumer cmd/txdetails/consumer/main.go

# API stage
FROM alpine:latest AS api
WORKDIR /app
COPY --from=builder /gosol-api ./gosol
COPY config/base.yaml ./config/
COPY config/api.yaml ./config/
CMD ["./gosol"]

# Producer stage
FROM alpine:latest AS monitor-producer
WORKDIR /app
COPY --from=builder /gosol-monitor-producer ./gosol
COPY config/base.yaml ./config/
COPY config/monitor.yaml ./config/
CMD ["./gosol"]

# Consumer stage
FROM alpine:latest AS monitor-consumer
WORKDIR /app
COPY --from=builder /gosol-monitor-consumer ./gosol
COPY config/base.yaml ./config/
COPY config/monitor.yaml ./config/
CMD ["./gosol"]

# Producer stage
FROM alpine:latest AS txdetails-producer
WORKDIR /app
COPY --from=builder /gosol-txdetails-producer ./gosol
COPY config/base.yaml ./config/
COPY config/txdetails.yaml ./config/
CMD ["./gosol"]

# Consumer stage
FROM alpine:latest AS txdetails-consumer
WORKDIR /app
COPY --from=builder /gosol-txdetails-consumer ./gosol
COPY config/base.yaml ./config/
COPY config/txdetails.yaml ./config/
CMD ["./gosol"]