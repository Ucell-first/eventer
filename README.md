# Log Processor
A Go microservice that processes log messages from Kafka and stores them in ClickHouse.

## Quick Start

### 1. Run with Docker
```bash
docker-compose up -d
```

### 2. Run Locally
```bash
go mod tidy
go run cmd/main.go
```

## Configuration (.env)
```env
BROKER_URL=kafka:9092
TOPIC=logs
CLICKHOUSE_URL=clickhouse:9000  # but at web you can use http://localhost:8123
BATCH_SIZE=100
FLUSH_INTERVAL=3s
```

## Kafka Message Format
Send JSON messages to Kafka topic:
```json
{
  "timestamp": "2025-07-17T14:00:00Z",
  "method": "GET",
  "path": "/api/users",
  "status": 200,
  "latency_ms": 8,
  "ip": "192.168.1.1"
}
```

## Send Test Message
```bash
# Create topic local
make create-topic

# Send message example
	@echo '{"timestamp":"2025-07-15T10:03:00Z","method":"DELETE","path":"/api/users/1","status":500,"latency_ms":1500,"ip":"192.168.1.103"}' | docker exec -i kafka-service-kafka-1 kafka-console-producer --topic logs --bootstrap-server localhost:9092
```

## API Endpoints
- `http://localhost:8081/metrics` - Prometheus metrics
- `http://localhost:9090` - Prometheus UI

## Ports
- `:8080` - Main application
- `:8081` - Metrics endpoint
- `:9090` - Prometheus UI

## Stop
```bash
docker-compose down
```
