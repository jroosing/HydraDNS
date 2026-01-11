# =============================================================================
# HydraDNS Makefile
# =============================================================================
# Common development tasks for testing and code quality

.PHONY: help test fmt vet build check clean docker-build docker-run docs

# Default target
help:
	@echo "HydraDNS Development Commands"
	@echo "=============================="
	@echo ""
	@echo "Testing:"
	@echo "  make test          Run all tests"
	@echo ""
	@echo "Code Quality:"
	@echo "  make fmt           Format Go code"
	@echo "  make vet           Run go vet"
	@echo "  make build         Build binaries"
	@echo "  make check         Run fmt + vet + test"
	@echo ""
	@echo "Documentation:"
	@echo "  make docs          Generate Swagger/OpenAPI docs"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build  Build Docker image"
	@echo "  make docker-run    Run with docker-compose"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean         Remove build artifacts and caches"

# =============================================================================
# Testing
# =============================================================================

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

build:
	go build ./cmd/hydradns

check: fmt vet test
	@echo "All checks passed!"

# =============================================================================
# Documentation
# =============================================================================

docs:
	@echo "Generating Swagger/OpenAPI documentation..."
	go run github.com/swaggo/swag/cmd/swag@latest init -g internal/api/handlers/base.go -o internal/api/docs --parseDependency --parseInternal
	@echo "Docs generated in internal/api/docs/"

# =============================================================================
# Docker
# =============================================================================

docker-build:
	docker build -t hydradns:latest .

docker-run:
	docker compose up

docker-down:
	docker compose down

# =============================================================================
# Cleanup
# =============================================================================

clean:
	rm -rf .coverage htmlcov
	find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
