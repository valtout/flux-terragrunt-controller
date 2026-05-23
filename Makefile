.PHONY: test

test:
	go test ./...

# =============================================================================
# E2E Testing targets (Kind-based)
# =============================================================================

E2E_SCRIPTS_DIR := scripts/e2e
CLUSTER_NAME ?= flux-terragrunt-e2e
K8S_VERSION ?= 1.36.0
NAMESPACE ?= flux-system
CONTROLLER_NAMESPACE ?= flux-terragrunt-controller

.PHONY: e2e-setup
e2e-setup: ## Create Kind cluster for E2E testing
	@echo "Setting up Kind cluster..."
	CLUSTER_NAME=$(CLUSTER_NAME) K8S_VERSION=$(K8S_VERSION) $(E2E_SCRIPTS_DIR)/setup-kind.sh

.PHONY: e2e-smoke
e2e-smoke: ## Run smoke tests against the Kind cluster
	@echo "Running smoke tests..."
	$(E2E_SCRIPTS_DIR)/run-smoke-tests.sh

.PHONY: e2e-deploy
e2e-deploy: ## Deploy the controller to the Kind cluster
	@echo "Deploying controllers..."
	CLUSTER_NAME=$(CLUSTER_NAME) NAMESPACE=$(NAMESPACE) CONTROLLER_NAMESPACE=$(CONTROLLER_NAMESPACE) \
		$(E2E_SCRIPTS_DIR)/deploy-controllers.sh

.PHONY: e2e-test
e2e-test: ## Run E2E tests against the Kind cluster
	@echo "Running E2E tests..."
	NAMESPACE=$(NAMESPACE) CONTROLLER_NAMESPACE=$(CONTROLLER_NAMESPACE) \
		$(E2E_SCRIPTS_DIR)/run-e2e-tests.sh

.PHONY: e2e-teardown
e2e-teardown: ## Delete the Kind cluster
	@echo "Tearing down Kind cluster..."
	CLUSTER_NAME=$(CLUSTER_NAME) $(E2E_SCRIPTS_DIR)/teardown-kind.sh

.PHONY: e2e
e2e: e2e-setup e2e-smoke e2e-deploy e2e-test ## Full E2E test cycle (setup, smoke, deploy, test)
	@echo "E2E tests completed"

.PHONY: e2e-full
e2e-full: e2e teardown ## Full E2E test cycle including teardown
	@echo "Full E2E cycle complete"

# =============================================================================
# Utility targets
# =============================================================================

.PHONY: kind-check
kind-check: ## Check if kind is installed
	@which kind > /dev/null || echo "kind is not installed. See https://kind.sigs.k8s.io/"

.PHONY: docker-check
docker-check: ## Check if docker is running
	@docker info > /dev/null || echo "Docker is not running"

.PHONY: show-cluster-status
show-cluster-status: ## Show current cluster status
	@kubectl cluster-info
	@kubectl get nodes
	@kubectl get all --all-namespaces
