module vektor-shipment/services/shipment-service

go 1.24.0

replace vektor-shipment => ../../

require (
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.11.2
	github.com/rabbitmq/amqp091-go v1.10.0
	google.golang.org/grpc v1.79.2
	google.golang.org/protobuf v1.36.11
	vektor-shipment v0.0.0-00010101000000-000000000000
)

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
)
