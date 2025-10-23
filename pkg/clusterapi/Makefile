# Minikube Cluster API Provider Makefile

# Image URL to use for building/pushing image targets
IMG ?= minikube-capi-provider:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: fmt vet ## Build manager binary.
	go build -o bin/minikube-capi-provider ../../cmd/minikube-capi-provider/main.go

.PHONY: run
run: fmt vet ## Run the controller from your host.
	go run ../../cmd/minikube-capi-provider/main.go \
		--storage-path=/var/lib/minikube \
		--profile=minikube

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build -f Dockerfile -t ${IMG} ../..

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

.PHONY: minikube-load
minikube-load: docker-build ## Build and load image into minikube.
	minikube image load ${IMG}

##@ Deployment

.PHONY: install-crd
install-crd: ## Install CRDs into the cluster.
	kubectl apply -f config/crd/

.PHONY: uninstall-crd
uninstall-crd: ## Uninstall CRDs from the cluster.
	kubectl delete -f config/crd/

.PHONY: install-rbac
install-rbac: ## Install RBAC into the cluster.
	kubectl apply -f config/rbac/

.PHONY: uninstall-rbac
uninstall-rbac: ## Uninstall RBAC from the cluster.
	kubectl delete -f config/rbac/

.PHONY: deploy
deploy: install-crd install-rbac ## Deploy controller to the cluster.
	kubectl apply -f config/manager/

.PHONY: undeploy
undeploy: ## Undeploy controller from the cluster.
	kubectl delete -f config/manager/
	kubectl delete -f config/rbac/
	kubectl delete -f config/crd/

.PHONY: install-all
install-all: docker-build minikube-load deploy ## Build, load image, and deploy everything.

##@ Examples

.PHONY: example-quickstart
example-quickstart: ## Apply quick-start example.
	kubectl apply -f examples/quick-start.yaml

.PHONY: example-machinedeployment
example-machinedeployment: ## Apply machinedeployment example.
	kubectl apply -f examples/machinedeployment.yaml

.PHONY: clean-examples
clean-examples: ## Clean up example resources.
	-kubectl delete -f examples/machinedeployment.yaml
	-kubectl delete -f examples/quick-start.yaml

##@ Cluster API Setup

.PHONY: install-clusterctl
install-clusterctl: ## Install clusterctl CLI.
	@echo "Installing clusterctl..."
	curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.10.0/clusterctl-linux-amd64 -o /tmp/clusterctl
	chmod +x /tmp/clusterctl
	sudo mv /tmp/clusterctl /usr/local/bin/
	@echo "Done! Run 'clusterctl version' to verify."

.PHONY: init-capi
init-capi: ## Initialize Cluster API in the cluster.
	clusterctl init

##@ Verification

.PHONY: verify-install
verify-install: ## Verify the installation.
	@echo "Checking CRDs..."
	kubectl get crds | grep minikube || echo "❌ CRDs not found"
	@echo ""
	@echo "Checking namespace..."
	kubectl get namespace minikube-capi-provider-system || echo "❌ Namespace not found"
	@echo ""
	@echo "Checking deployment..."
	kubectl get deployment -n minikube-capi-provider-system || echo "❌ Deployment not found"
	@echo ""
	@echo "Checking pod status..."
	kubectl get pods -n minikube-capi-provider-system

.PHONY: logs
logs: ## Show controller logs.
	kubectl logs -n minikube-capi-provider-system -l control-plane=controller-manager --tail=100 -f

.PHONY: status
status: ## Show status of provider and resources.
	@echo "=== Provider Status ==="
	kubectl get pods -n minikube-capi-provider-system
	@echo ""
	@echo "=== MinikubeClusters ==="
	kubectl get minikubeclusters --all-namespaces
	@echo ""
	@echo "=== MinikubeMachines ==="
	kubectl get minikubemachines --all-namespaces
	@echo ""
	@echo "=== Nodes ==="
	kubectl get nodes

##@ Cleanup

.PHONY: clean
clean: clean-examples undeploy ## Clean up all resources.
	@echo "Cleanup complete!"

.PHONY: clean-all
clean-all: clean ## Clean everything including binary.
	rm -f bin/minikube-capi-provider
	docker rmi ${IMG} || true
