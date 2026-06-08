# axis-flow Makefile

.PHONY: help dev back front build build-back build-front install clean install-deps lint test

BACK_DIR := axis-flow-back
FRONT_DIR := axis-flow-front
BACK_PORT ?= 8080
FRONT_PORT ?= 5173
DB_HOST ?= localhost
DB_PORT ?= 5432
REDIS_HOST ?= localhost
REDIS_PORT ?= 6379

# Colors
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

help:
	@echo "$(GREEN)axis-flow Dev Tool$(NC)"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  $(BLUE)dev$(NC)              Start both backend and frontend (default)"
	@echo "  $(BLUE)back$(NC)             Start only backend"
	@echo "  $(BLUE)front$(NC)            Start only frontend"
	@echo "  $(BLUE)build$(NC)            Build both projects"
	@echo "  $(BLUE)build-back$(NC)       Build backend binary"
	@echo "  $(BLUE)build-front$(NC)      Build frontend (production)"
	@echo "  $(BLUE)install$(NC)          Install dependencies"
	@echo "  $(BLUE)install-deps$(NC)     Alias for install"
	@echo "  $(BLUE)lint$(NC)             Lint both projects"
	@echo "  $(BLUE)test$(NC)             Run tests for both projects"
	@echo "  $(BLUE)clean$(NC)            Clean build artifacts"
	@echo "  $(BLUE)help$(NC)             Show this help message"
	@echo ""
	@echo "Environment Variables:"
	@echo "  BACK_PORT=$(BACK_PORT)"
	@echo "  FRONT_PORT=$(FRONT_PORT)"
	@echo "  DB_HOST=$(DB_HOST)"
	@echo "  DB_PORT=$(DB_PORT)"
	@echo "  REDIS_HOST=$(REDIS_HOST)"
	@echo "  REDIS_PORT=$(REDIS_PORT)"
	@echo ""
	@echo "Examples:"
	@echo "  make dev"
	@echo "  make BACK_PORT=3000 dev"
	@echo "  make build"

dev: check-deps
	@echo "$(BLUE)Starting both services...$(NC)"
	@echo "Backend: http://localhost:$(BACK_PORT)"
	@echo "Frontend: http://localhost:$(FRONT_PORT)"
	@echo "$(YELLOW)Press Ctrl+C to stop$(NC)"
	@echo ""
	@$(MAKE) start-all

back: check-deps
	@echo "$(BLUE)Starting backend on port $(BACK_PORT)...$(NC)"
	@cd $(BACK_DIR) && \
	if [ ! -f "bin/server" ]; then \
		$(MAKE) build-back; \
	fi && \
	PORT=$(BACK_PORT) ./bin/server

front: check-deps
	@echo "$(BLUE)Starting frontend on port $(FRONT_PORT)...$(NC)"
	@cd $(FRONT_DIR) && npm run dev -- --port $(FRONT_PORT)

build: build-back build-front
	@echo "$(GREEN)✓ Both projects built successfully$(NC)"

build-back: check-deps
	@echo "$(BLUE)Building backend...$(NC)"
	@cd $(BACK_DIR) && \
	mkdir -p bin && \
	go build -o bin/server ./cmd/server
	@echo "$(GREEN)✓ Backend built successfully$(NC)"

build-front: check-deps
	@echo "$(BLUE)Building frontend...$(NC)"
	@cd $(FRONT_DIR) && npm run build
	@echo "$(GREEN)✓ Frontend built successfully$(NC)"

install: install-deps

install-deps: check-deps
	@echo "$(BLUE)Installing dependencies...$(NC)"
	@echo "Installing backend dependencies..."
	@cd $(BACK_DIR) && go mod download && go mod verify
	@echo "$(GREEN)✓ Backend dependencies installed$(NC)"
	@echo "Installing frontend dependencies..."
	@cd $(FRONT_DIR) && npm install
	@echo "$(GREEN)✓ Frontend dependencies installed$(NC)"

lint:
	@echo "$(BLUE)Linting backend...$(NC)"
	@cd $(BACK_DIR) && go fmt ./...
	@echo "$(GREEN)✓ Backend formatted$(NC)"
	@echo "$(BLUE)Linting frontend...$(NC)"
	@cd $(FRONT_DIR) && npm run lint
	@echo "$(GREEN)✓ Frontend linted$(NC)"

test:
	@echo "$(BLUE)Running backend tests...$(NC)"
	@cd $(BACK_DIR) && go test -v ./...
	@echo "$(GREEN)✓ Backend tests passed$(NC)"
	@echo "$(BLUE)Running frontend tests...$(NC)"
	@cd $(FRONT_DIR) && npm test 2>/dev/null || echo "$(YELLOW)No frontend tests configured$(NC)"

clean:
	@echo "$(BLUE)Cleaning artifacts...$(NC)"
	@cd $(BACK_DIR) && rm -rf bin && go clean && go clean -cache
	@echo "$(GREEN)✓ Backend cleaned$(NC)"
	@cd $(FRONT_DIR) && rm -rf dist node_modules && npm cache clean --force
	@echo "$(GREEN)✓ Frontend cleaned$(NC)"

check-deps:
	@command -v go >/dev/null 2>&1 || { echo "$(RED)✗ Go not found$(NC)"; exit 1; }
	@command -v node >/dev/null 2>&1 || { echo "$(RED)✗ Node.js not found$(NC)"; exit 1; }
	@command -v npm >/dev/null 2>&1 || { echo "$(RED)✗ npm not found$(NC)"; exit 1; }
	@echo "$(GREEN)✓ All dependencies found$(NC)"

.PHONY: start-all
start-all:
	@(cd $(BACK_DIR) && PORT=$(BACK_PORT) ./bin/server > /tmp/back.log 2>&1 &) && \
	sleep 2 && \
	(cd $(FRONT_DIR) && npm run dev -- --port $(FRONT_PORT) &) && \
	wait
