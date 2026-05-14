.PHONY: help build build-linux build-docker test clean deploy logs

# Color output
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m # No Color

BINARY_NAME := monitor-agent
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

help:
	@echo "$(GREEN)Monitor Agent - Build Commands$(NC)"
	@echo ""
	@echo "$(YELLOW)Build:$(NC)"
	@echo "  make build              Build for current OS"
	@echo "  make build-linux        Build for Linux (amd64)"
	@echo "  make build-docker       Build Docker image"
	@echo ""
	@echo "$(YELLOW)Testing:$(NC)"
	@echo "  make test               Run tests"
	@echo "  make bench              Run benchmarks"
	@echo ""
	@echo "$(YELLOW)Development:$(NC)"
	@echo "  make fmt                Format code"
	@echo "  make lint               Run linter"
	@echo "  make check              Run security checks"
	@echo ""
	@echo "$(YELLOW)Deployment:$(NC)"
	@echo "  make deploy-docker      Deploy with Docker Compose"
	@echo "  make deploy-systemd     Deploy as systemd service"
	@echo "  make deploy-k8s         Deploy to Kubernetes"
	@echo ""
	@echo "$(YELLOW)Utilities:$(NC)"
	@echo "  make clean              Clean build artifacts"
	@echo "  make logs               Tail agent logs"
	@echo "  make config             Validate configuration"
	@echo "  make help               Show this help"
	@echo ""

build:
	@echo "$(GREEN)Building Monitor Agent $(VERSION)$(NC)"
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/agent
	@echo "$(GREEN)✓ Built $(BINARY_NAME)$(NC)"

build-linux:
	@echo "$(GREEN)Building for Linux amd64 $(VERSION)$(NC)"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) \
		-o $(BINARY_NAME)-linux-amd64 ./cmd/agent
	@echo "$(GREEN)✓ Built $(BINARY_NAME)-linux-amd64$(NC)"

build-docker:
	@echo "$(GREEN)Building Docker image$(NC)"
	docker build -f deployments/docker/Dockerfile \
		-t monitor-agent:$(VERSION) \
		-t monitor-agent:latest \
		.
	@echo "$(GREEN)✓ Docker image built$(NC)"

test:
	@echo "$(GREEN)Running tests$(NC)"
	go test -v -race -timeout 30s ./...
	@echo "$(GREEN)✓ Tests passed$(NC)"

bench:
	@echo "$(GREEN)Running benchmarks$(NC)"
	go test -bench=. -benchmem -benchtime=10s ./internal/queue/
	@echo "$(GREEN)✓ Benchmarks complete$(NC)"

fmt:
	@echo "$(GREEN)Formatting code$(NC)"
	go fmt ./...
	@echo "$(GREEN)✓ Code formatted$(NC)"

lint:
	@echo "$(GREEN)Running linter$(NC)"
	golangci-lint run ./...
	@echo "$(GREEN)✓ Linter passed$(NC)"

check:
	@echo "$(GREEN)Running security checks$(NC)"
	gosec ./...
	@echo "$(GREEN)✓ Security checks passed$(NC)"

clean:
	@echo "$(GREEN)Cleaning build artifacts$(NC)"
	rm -f $(BINARY_NAME) $(BINARY_NAME)-*
	go clean -testcache ./...
	@echo "$(GREEN)✓ Clean complete$(NC)"

config:
	@echo "$(GREEN)Validating configuration$(NC)"
	@if [ -f config.json ]; then \
		jq . config.json > /dev/null && echo "$(GREEN)✓ Valid JSON$(NC)" || exit 1; \
	else \
		echo "$(RED)✗ config.json not found$(NC)"; exit 1; \
	fi

deploy-docker:
	@echo "$(GREEN)Deploying with Docker Compose$(NC)"
	docker-compose -f deployments/docker/docker-compose.yml up -d
	@echo "$(GREEN)✓ Deployed$(NC)"
	@echo "View logs: docker-compose -f deployments/docker/docker-compose.yml logs -f"

deploy-systemd:
	@echo "$(GREEN)Deploying as systemd service$(NC)"
	@if ! command -v systemctl &> /dev/null; then \
		echo "$(RED)✗ systemctl not found$(NC)"; exit 1; \
	fi
	@echo "$(YELLOW)Running as root for installation$(NC)"
	sudo cp $(BINARY_NAME) /usr/local/bin/
	sudo chmod 755 /usr/local/bin/$(BINARY_NAME)
	sudo useradd -r -s /bin/false monitor || true
	sudo mkdir -p /etc/monitor-agent /var/lib/monitor-agent
	sudo chown monitor:monitor /var/lib/monitor-agent
	sudo chmod 700 /var/lib/monitor-agent
	@echo "$(GREEN)✓ Agent installed$(NC)"
	@echo "$(YELLOW)Next: sudo cp config.json /etc/monitor-agent/$(NC)"
	@echo "$(YELLOW)Then: sudo systemctl start monitor-agent$(NC)"

deploy-k8s:
	@echo "$(GREEN)Deploying to Kubernetes$(NC)"
	@if ! command -v kubectl &> /dev/null; then \
		echo "$(RED)✗ kubectl not found$(NC)"; exit 1; \
	fi
	kubectl create namespace monitoring || true
	kubectl create secret generic monitor-token \
		--from-literal=token=project_CHANGE_ME \
		-n monitoring || true
	kubectl create configmap monitor-config \
		--from-file=config.json=config/examples/config.json \
		-n monitoring || true
	kubectl apply -f deployments/kubernetes/
	@echo "$(GREEN)✓ Deployed to Kubernetes$(NC)"
	@echo "$(YELLOW)View pods: kubectl get pods -n monitoring$(NC)"

logs:
	@echo "$(GREEN)Tailing agent logs$(NC)"
	@if command -v journalctl &> /dev/null; then \
		sudo journalctl -u monitor-agent -f; \
	elif command -v docker &> /dev/null; then \
		docker-compose -f deployments/docker/docker-compose.yml logs -f; \
	else \
		echo "$(RED)✗ No log viewer found$(NC)"; exit 1; \
	fi

dev:
	@echo "$(GREEN)Starting development environment$(NC)"
	@echo "1. Install dependencies:"
	@echo "   go mod download"
	@echo "2. Build agent:"
	@echo "   make build"
	@echo "3. Configure:"
	@echo "   cp config/examples/config.json ./config.json"
	@echo "   # Edit with your token"
	@echo "4. Run:"
	@echo "   ./$(BINARY_NAME)"

version:
	@echo "Monitor Agent $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date: $(DATE)"

install-tools:
	@echo "$(GREEN)Installing development tools$(NC)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "$(GREEN)✓ Tools installed$(NC)"

docker-push:
	@echo "$(GREEN)Pushing Docker image$(NC)"
	docker push monitor-agent:$(VERSION)
	docker push monitor-agent:latest
	@echo "$(GREEN)✓ Pushed$(NC)"

.PHONY: help build build-linux build-docker test bench fmt lint check clean config deploy-docker deploy-systemd deploy-k8s logs dev version install-tools docker-push
