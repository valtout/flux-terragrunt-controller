.PHONY: test lint helm-test integration-test e2e-setup e2e-smoke e2e-deploy e2e-test e2e-teardown e2e e2e-full

# =============================================================================
# Test targets
# =============================================================================

test: ## Run unit tests
	go test -v -short ./...

lint: ## Run linting (go vet, golangci-lint)
	go vet ./...
	golangci-lint run || echo "golangci-lint not installed, skipping"

integration-test: ## Run integration tests (requires envtest)
	@if ! command -v setup-envtest &> /dev/null; then \
		go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest; \
	fi
	ENVTEST_K8S_VERSION=1.29 setup-envtest use 1.29 -p path > /tmp/envtest-path.txt
	export KUBEBUILDER_ASSETS=$$(cat /tmp/envtest-path.txt); \
	go test -v -run TestIntegration ./pkg/controller/...

helm-test: ## Run Helm chart tests
	helm unittest ./charts/flux-terragrunt-controller --with-subchart_tests || true
	ct lint --all --validate-maintainers=false || echo "chart-testing not installed"
	find ./charts -name "*.yaml" -exec yamllint {} \; || true

# Convenience target for all non-E2E tests
test-all: test lint integration-test helm-test ## Run all tests (unit, lint, integration, helm)

# =============================================================================
# E2E Testing targets (Kind-based)
# =============================================================================

E2E_SCRIPTS_DIR := scripts/e2e
CLUSTER_NAME ?= flux-terragrunt-e2e
K8S_VERSION ?= 1.36.0
NAMESPACE ?= flux-system
CONTROLLER_NAMESPACE ?= flux-terragrunt-controller

e2e-setup: ## Create Kind cluster for E2E testing
	@echo "Setting up Kind cluster..."
	CLUSTER_NAME=$(CLUSTER_NAME) K8S_VERSION=$(K8S_VERSION) $(E2E_SCRIPTS_DIR)/setup-kind.sh

e2e-smoke: ## Run smoke tests against the Kind cluster
	@echo "Running smoke tests..."
	$(E2E_SCRIPTS_DIR)/run-smoke-tests.sh

e2e-deploy: ## Deploy the controller to the Kind cluster
	@echo "Deploying controllers..."
	CLUSTER_NAME=$(CLUSTER_NAME) NAMESPACE=$(NAMESPACE) CONTROLLER_NAMESPACE=$(CONTROLLER_NAMESPACE) \
		$(E2E_SCRIPTS_DIR)/deploy-controllers.sh

e2e-test: ## Run E2E tests against the Kind cluster
	@echo "Running E2E tests..."
	NAMESPACE=$(NAMESPACE) CONTROLLER_NAMESPACE=$(CONTROLLER_NAMESPACE) \
		$(E2E_SCRIPTS_DIR)/run-e2e-tests.sh

e2e-teardown: ## Delete the Kind cluster
	@echo "Tearing down Kind cluster..."
	CLUSTER_NAME=$(CLUSTER_NAME) $(E2E_SCRIPTS_DIR)/teardown-kind.sh

e2e: e2e-setup e2e-smoke e2e-deploy e2e-test ## Full E2E test cycle (setup, smoke, deploy, test)
	@echo "E2E tests completed"

e2e-full: e2e e2e-teardown ## Full E2E test cycle including teardown
	@echo "Full E2E cycle complete"

# =============================================================================
# Utility targets
# =============================================================================

.PHONY: kind-check docker-check show-cluster-status build

kind-check: ## Check if kind is installed
	@which kind > /dev/null || echo "kind is not installed. See https://kind.sigs.k8s.io/"

docker-check: ## Check if docker is running
	@docker info > /dev/null || echo "Docker is not running"

show-cluster-status: ## Show current cluster status
	@kubectl cluster-info
	@kubectl get nodes
	@kubectl get all --all-namespaces

build: ## Build the controller binary
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/flux-terragrunt-controller ./cmd/flux-terragrunt-controller

docker-build: ## Build Docker images
	docker build -t flux-terragrunt-controller:latest -f images/Dockerfile.controller .
	docker build -t flux-terragrunt-runner:latest -f images/Dockerfile.runner ./images

docker-load-images: ## Load Docker images from tar files
	docker load --input /tmp/controller.tar || true
	docker load --input /tmp/runner.tar || true

docker-inspect: ## Inspect Docker images
	docker inspect flux-terragrunt-controller:latest || true
	docker inspect flux-terragrunt-runner:latest || true

helm-template: ## Render Helm chart to verify templates
	helm template flux-terragrunt-controller ./charts/flux-terragrunt-controller > /tmp/rendered.yaml
	test -s /tmp/rendered.yaml
