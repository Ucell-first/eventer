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
