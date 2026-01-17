# =============================================================================
# HydraDNS Makefile
# =============================================================================
# Common development tasks for testing and code quality

.PHONY: help test fmt vet build build-ui-embedded check clean docker-build docker-run docker-down docs ui-build ui-serve ui-clean docker-build-ui run

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
	@echo "  make build-ui-embedded  Build with embedded Angular UI"
	@echo "  make run           Run the built binary with default database"
	@echo "  make check         Run fmt + vet + test"
	@echo ""
	@echo "Documentation:"
	@echo "  make docs          Generate Swagger/OpenAPI docs"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build  Build Docker image"
	@echo "  make docker-run    Run with docker-compose"
	@echo "  make docker-build-ui  Build Docker image with embedded UI (Dockerfile.ui)"
	@echo ""
	@echo "Frontend (Angular):"
	@echo "  make ui-build      Build Angular UI (ui/hydradns)"
	@echo "  make ui-serve      Serve Angular dev server with proxy"
	@echo "  make ui-clean      Remove Angular dist build output"
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

build: ui-build
	@echo "Copying Angular dist to internal/api/dist..."
	mkdir -p internal/api/dist
	rm -rf internal/api/dist/*
	cp -R ui/hydradns/dist/hydradns/* internal/api/dist/
	@echo "Building Go binary with embedded UI..."
	mkdir -p bin
	go build -o bin/hydradns ./cmd/hydradns
	@echo "Done! Binary: bin/hydradns (with embedded UI)"

build-no-fe:
	mkdir -p bin
	go build -o bin/hydradns ./cmd/hydradns

run: build
	./bin/hydradns

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

docker-build-ui:
	docker build -f Dockerfile.ui -t hydradns:ui .

# =============================================================================
# Cleanup
# =============================================================================

clean:
	rm -rf bin .coverage htmlcov
	find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true

# =============================================================================
# Frontend (Angular)
# =============================================================================

ui-build:
	cd ui/hydradns && npm ci && npm run build -- --configuration production

ui-serve:
	cd ui/hydradns && npm start -- --proxy-config proxy.conf.json

ui-clean:
	rm -rf ui/hydradns/dist
