SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

# Strata v2 — top-level dev workflow. See AGENTS.md for the full plan.

.DEFAULT_GOAL := help

.PHONY: help
help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

# ── TUI ───────────────────────────────────────────────────────────

.PHONY: tui-dev
tui-dev: ## install + run the Strata TUI (uv)
	cd tui && uv sync --all-extras && uv run strata

.PHONY: tui-test
tui-test: ## run TUI pytest suite
	cd tui && uv sync --all-extras && uv run --all-extras pytest

.PHONY: tui-lint
tui-lint: ## ruff lint the TUI
	cd tui && uv sync --all-extras && uv run --all-extras ruff check strata_tui tests

# ── Backend Go services ────────────────────────────────────────────

.PHONY: backend-test
backend-test: ## run Go tests for orchestrator + shared
	cd backend/services/shared && go test ./...
	cd backend/services/orchestrator && go test ./...

.PHONY: backend-lint
backend-lint: ## vet + format backend Go modules
	cd backend/services/shared && go vet ./... && gofmt -l .
	cd backend/services/orchestrator && go vet ./... && gofmt -l .

.PHONY: orchestrator-image
orchestrator-image: ## build the orchestrator Docker image
	docker build -t strata-orchestrator:dev -f backend/services/orchestrator/Dockerfile .

.PHONY: mcp-k8s-image
mcp-k8s-image: ## build the MCP k8s server Docker image
	docker build -t strata-mcp-k8s:dev -f backend/mcp-servers/k8s/Dockerfile backend/mcp-servers/k8s

# ── MCP servers (Python) ───────────────────────────────────────────

.PHONY: mcp-k8s-test
mcp-k8s-test: ## run pytest suite for MCP k8s server
	cd backend/mcp-servers/k8s && uv sync --all-extras && uv run --all-extras pytest

.PHONY: mcp-k8s-lint
mcp-k8s-lint: ## ruff lint the MCP k8s server
	cd backend/mcp-servers/k8s && uv sync --all-extras && uv run --all-extras ruff check strata_mcp_k8s tests

# ── Local kind cluster ─────────────────────────────────────────────

.PHONY: kind-up
kind-up: ## create the strata-dev kind cluster
	kind create cluster --config infra/kind/strata-dev.yaml
	@echo "kind cluster ready. Get kubeconfig with: kind get kubeconfig --name strata-dev"

.PHONY: kind-down
kind-down: ## delete the strata-dev kind cluster
	kind delete cluster --name strata-dev

.PHONY: platform-deps
platform-deps: ## install Envoy Gateway into the running kind cluster
	./scripts/install-platform-deps.sh

.PHONY: backend-up
backend-up: orchestrator-image mcp-k8s-image ## build images, load into kind, helm install Strata
	kind load docker-image strata-orchestrator:dev --name strata-dev
	kind load docker-image strata-mcp-k8s:dev --name strata-dev
	helm upgrade --install strata backend/helm/strata \
	    --namespace strata --create-namespace \
	    -f backend/helm/strata/values-kind.yaml

.PHONY: backend-down
backend-down: ## helm uninstall Strata
	helm uninstall strata --namespace strata || true

.PHONY: backend-logs
backend-logs: ## tail logs across all strata pods
	kubectl logs -l app.kubernetes.io/part-of=strata -n strata --tail=100 -f

.PHONY: e2e
e2e: ## full Phase 1 end-to-end smoke (kind + helm + REST API check)
	./scripts/e2e.sh

# ── Reset ──────────────────────────────────────────────────────────

.PHONY: reset
reset: ## nuke caches, .venv, build artifacts
	@find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
	@find . -type d -name .pytest_cache -exec rm -rf {} + 2>/dev/null || true
	@find . -type d -name .ruff_cache -exec rm -rf {} + 2>/dev/null || true
	@rm -rf tui/.venv tui/uv.lock 2>/dev/null || true
	@rm -rf backend/mcp-servers/k8s/.venv backend/mcp-servers/k8s/uv.lock 2>/dev/null || true
	@echo "reset complete"