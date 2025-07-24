include .env
export $(shell sed 's/=.*//' .env)

CURRENT_DIR=$(shell pwd)

.PHONY: proto-gen
proto-gen:
	./scripts/gen-proto.sh ${CURRENT_DIR}

run:
	go run ./cmd

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


docker-up:
	docker-compose up --build -d

docker-stop:
	docker-compose down

docker-logs:
	docker compose logs -f kafka-service-eventer-1


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
	docker exec kafka-service-clickhouse-1 clickhouse-client --user $(CLICKHOUSE_USER) --password $(CLICKHOUSE_PASSWORD) --query "SELECT * FROM logs ORDER BY timestamp DESC LIMIT 10"

check-logs-curl:
	curl -u default:12345 "http://localhost:8123/?query=SELECT%20*%20FROM%20logs%20ORDER%20BY%20timestamp%20DESC%20LIMIT%2010"

start-all: docker-up
	@echo "⏳ Waiting for services to start..."
	sleep 10
	@echo "📋 Creating Kafka topic..."
	$(MAKE) create-topic
	@echo "📤 Sending test logs..."
	$(MAKE) send-test-logs
	@echo "✅ All services started and test data sent!"

enter-service:
	docker exec -it kafka-service-eventer-1 /bin/sh
