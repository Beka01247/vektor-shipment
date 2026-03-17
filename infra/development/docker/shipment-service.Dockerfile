FROM golang:1.23-alpine AS builder

WORKDIR /build

COPY build/shipment-service /app/build/shipment-service
COPY shared /app/shared

WORKDIR /app

RUN chmod +x /app/build/shipment-service

FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/build/shipment-service /app/build/shipment-service

EXPOSE 50052

ENTRYPOINT ["/app/build/shipment-service"]
