# =============================================================================
# HydraDNS Makefile
# =============================================================================
# Common development tasks for testing and code quality

.PHONY: help test fmt vet build build-ui-embedded check clean docs ui-build ui-serve ui-clean run

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
	@echo "Frontend (Angular):"
	@echo "  make ui-build      Fetch & Build Angular UI (from external repo)"
	@echo "  make ui-clean      Remove fetched UI repository and build artifacts"
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
	# Assuming standard Angular build output structure
	cp -R $(UI_DIR)/dist/hydradns/* internal/api/dist/
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
# Cleanup
# =============================================================================

clean:
	rm -rf bin .coverage htmlcov
	find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true

# =============================================================================
# Frontend (Angular)
# =============================================================================

UI_REPO := https://github.com/jroosing/HydraDNS-frontend.git
UI_DIR := .ui-repo

ui-fetch:
	@if [ -d "$(UI_DIR)/.git" ]; then \
		echo "Pulling latest UI changes..."; \
		cd $(UI_DIR) && git pull; \
	else \
		echo "Cloning UI repository..."; \
		rm -rf $(UI_DIR); \
		git clone $(UI_REPO) $(UI_DIR); \
	fi

ui-build: ui-fetch
	cd $(UI_DIR) && npm ci && npm run build -- --configuration production

ui-serve: ui-fetch
	cd $(UI_DIR) && npm start -- --proxy-config proxy.conf.json

ui-clean:
	rm -rf $(UI_DIR)
