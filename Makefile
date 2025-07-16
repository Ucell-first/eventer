include .env
export $(shell sed 's/=.*//' .env)

CURRENT_DIR=$(shell pwd)

.PHONY: proto-gen
proto-gen:
	./scripts/gen-proto.sh ${CURRENT_DIR}

.PHONY: run
run:/Users/nurmuxammad/go/src/github.com/Ucell/auth/.env
	go run cmd/main.go

.PHONY: git
git:
	@echo "Enter commit name: "; \
	read commitname; \
	git add .; \
	git commit -m "$$commitname"; \
	if ! git push origin main; then \
		echo "Push failed. Attempting to merge and retry..."; \
		$(MAKE) git-merge; \
		git add .; \
		git commit -m "$$commitname"; \
		git push origin main; \
	fi

.PHONY: git-merge
git-merge:
	git fetch origin; \
	git merge origin/main

.PHONY: lint
lint:
	golangci-lint run


help:
	@echo "📚 Available commands:"
	@echo "  build          - Build the application"
	@echo "  run            - Run the application"
	@echo "  test           - Run tests"
	@echo "  clean          - Clean build artifacts"
	@echo "  docker-up      - Start Docker services"
	@echo "  docker-down    - Stop Docker services"
	@echo "  docker-build   - Build and start with Docker"
	@echo "  send-test-logs - Send test logs to Kafka"
	@echo "  create-topic   - Create Kafka topic"
	@echo "  check-logs     - Check logs in ClickHouse"
	@echo "  check-metrics  - Check Prometheus metrics"
	@echo "  health         - Check service health"
	@echo "  api-logs       - Get logs via API"
	@echo "  api-stats      - Get stats via API"
	@echo "  start-all      - Start everything and send test data"
	@echo "  help           - Show this help"


docker-up:
	docker-compose up -d

docker-stop:
	docker-compose down

docker-logs:
	docker-compose logs -f log-processor

test:
	go run test/producer.go

send-test-logs:
	@echo "📤 Sending test logs to Kafka..."
	@echo '{"timestamp":"2025-07-15T10:00:00Z","method":"GET","path":"/api/users","status":200,"latency_ms":45,"ip":"192.168.1.100"}' | docker exec -i kafka-service-kafka-1 kafka-console-producer --topic logs --bootstrap-server localhost:9092
	@echo '{"timestamp":"2025-07-15T10:01:00Z","method":"POST","path":"/api/users","status":201,"latency_ms":120,"ip":"192.168.1.101"}' | docker exec -i kafka-service-kafka-1 kafka-console-producer --topic logs --bootstrap-server localhost:9092
	@echo '{"timestamp":"2025-07-15T10:02:00Z","method":"GET","path":"/api/users/1","status":404,"latency_ms":25,"ip":"192.168.1.102"}' | docker exec -i kafka-service-kafka-1 kafka-console-producer --topic logs --bootstrap-server localhost:9092
	@echo '{"timestamp":"2025-07-15T10:03:00Z","method":"DELETE","path":"/api/users/1","status":500,"latency_ms":1500,"ip":"192.168.1.103"}' | docker exec -i kafka-service-kafka-1 kafka-console-producer --topic logs --bootstrap-server localhost:9092
	@echo "✅ Test logs sent successfully!"

create-topic:
	@echo "📋 Creating Kafka topic..."
	@docker exec kafka-service-kafka-1 kafka-topics --create --topic logs --bootstrap-server localhost:9092 --replication-factor 1 --partitions 1 2>&1 | tee /dev/stderr | grep -q "already exists" || true

check-logs:
	@echo "🔍 Checking logs in ClickHouse..."
	docker exec kafka-service-clickhouse-1 clickhouse-client --query "SELECT * FROM logs ORDER BY timestamp DESC LIMIT 10"

check-metrics:
	@echo "📈 Checking Prometheus metrics..."
	curl -s http://localhost:8081/metrics | grep log_processor

api-logs:
	@echo "🔍 Getting logs via API..."
	curl -s http://localhost:8080/api/v1/logs | jq .

start-all: docker-up
	@echo "⏳ Waiting for services to start..."
	sleep 10
	@echo "📋 Creating Kafka topic..."
	$(MAKE) create-topic
	@echo "📤 Sending test logs..."
	$(MAKE) send-test-logs
	@echo "✅ All services started and test data sent!"
