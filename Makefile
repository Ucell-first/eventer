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
