#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get the script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
BACK_DIR="$SCRIPT_DIR/axis-flow-back"
FRONT_DIR="$SCRIPT_DIR/axis-flow-front"

# Default mode
MODE=${1:-help}
VERBOSE=${VERBOSE:-0}

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

show_help() {
    cat << EOF
${GREEN}axis-flow Dev Tool${NC}

Usage: ./dev.sh <command> [options]

Commands:
  ${BLUE}dev${NC}              Start both backend and frontend (default)
  ${BLUE}back${NC}             Start only backend
  ${BLUE}front${NC}            Start only frontend
  ${BLUE}build-back${NC}       Build backend binary
  ${BLUE}build-front${NC}      Build frontend (production)
  ${BLUE}build${NC}            Build both projects
  ${BLUE}install${NC}          Install dependencies (both projects)
  ${BLUE}clean${NC}            Clean build artifacts
  ${BLUE}help${NC}             Show this help message

Options:
  ${BLUE}-v, --verbose${NC}    Show detailed output

Examples:
  ./dev.sh dev              # Start both services
  ./dev.sh back             # Start only backend
  ./dev.sh front            # Start only frontend
  ./dev.sh build            # Build both
  ./dev.sh -v dev           # Verbose output

Environment Variables:
  ${BLUE}BACK_PORT${NC}         Backend port (default: 8080)
  ${BLUE}FRONT_PORT${NC}        Frontend port (default: 5173)
  ${BLUE}DB_HOST${NC}           Database host (default: localhost)
  ${BLUE}DB_PORT${NC}           Database port (default: 5432)
  ${BLUE}REDIS_HOST${NC}        Redis host (default: localhost)
  ${BLUE}REDIS_PORT${NC}        Redis port (default: 6379)

EOF
}

check_dependencies() {
    local missing=0

    # Check Go
    if ! command -v go &> /dev/null; then
        log_error "Go not found. Please install Go 1.25.0 or later"
        missing=1
    else
        GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        log_success "Go found: $GO_VERSION"
    fi

    # Check Node.js
    if ! command -v node &> /dev/null; then
        log_error "Node.js not found. Please install Node.js"
        missing=1
    else
        NODE_VERSION=$(node --version)
        log_success "Node.js found: $NODE_VERSION"
    fi

    # Check npm
    if ! command -v npm &> /dev/null; then
        log_error "npm not found. Please install npm"
        missing=1
    else
        NPM_VERSION=$(npm --version)
        log_success "npm found: $NPM_VERSION"
    fi

    if [ $missing -eq 1 ]; then
        log_error "Missing dependencies. Please install the tools above."
        exit 1
    fi
}

install_dependencies() {
    log_info "Installing dependencies..."

    # Backend dependencies
    log_info "Installing backend dependencies..."
    cd "$BACK_DIR"
    go mod download
    go mod verify
    log_success "Backend dependencies installed"

    # Frontend dependencies
    log_info "Installing frontend dependencies..."
    cd "$FRONT_DIR"
    npm install
    log_success "Frontend dependencies installed"

    log_success "All dependencies installed"
}

build_backend() {
    log_info "Building backend..."
    cd "$BACK_DIR"

    if go build -o bin/server ./cmd/server; then
        log_success "Backend built successfully"
    else
        log_error "Backend build failed"
        exit 1
    fi
}

build_frontend() {
    log_info "Building frontend..."
    cd "$FRONT_DIR"

    if npm run build; then
        log_success "Frontend built successfully"
    else
        log_error "Frontend build failed"
        exit 1
    fi
}

build_all() {
    build_backend
    build_frontend
}

start_backend() {
    local port=${BACK_PORT:-8080}
    log_info "Starting backend on port $port..."
    cd "$BACK_DIR"

    if [ ! -f "bin/server" ]; then
        log_warning "Backend binary not found. Building..."
        build_backend
    fi

    export PORT=$port
    ./bin/server
}

start_frontend() {
    local port=${FRONT_PORT:-5173}
    log_info "Starting frontend on port $port..."
    cd "$FRONT_DIR"

    npm run dev -- --port $port
}

start_all() {
    local back_port=${BACK_PORT:-8080}
    local front_port=${FRONT_PORT:-5173}

    log_info "Starting both services..."
    log_info "Backend will run on port $back_port"
    log_info "Frontend will run on port $front_port"
    echo ""

    # Start backend in background
    (
        cd "$BACK_DIR"
        if [ ! -f "bin/server" ]; then
            build_backend
        fi
        export PORT=$back_port
        exec ./bin/server
    ) &
    BACK_PID=$!

    # Give backend time to start
    sleep 2

    # Start frontend in background
    (
        cd "$FRONT_DIR"
        exec npm run dev -- --port $front_port
    ) &
    FRONT_PID=$!

    log_success "Backend started (PID: $BACK_PID)"
    log_success "Frontend started (PID: $FRONT_PID)"
    echo ""
    log_info "Press Ctrl+C to stop both services"

    # Wait for both processes and handle signals
    trap "kill $BACK_PID $FRONT_PID 2>/dev/null; log_info 'Services stopped'" EXIT
    wait
}

clean() {
    log_info "Cleaning build artifacts..."

    # Clean backend
    log_info "Cleaning backend..."
    cd "$BACK_DIR"
    if [ -d "bin" ]; then
        rm -rf bin
        log_success "Backend cleaned"
    fi
    go clean
    go clean -cache

    # Clean frontend
    log_info "Cleaning frontend..."
    cd "$FRONT_DIR"
    if [ -d "dist" ]; then
        rm -rf dist
    fi
    if [ -d "node_modules" ]; then
        rm -rf node_modules
    fi
    npm cache clean --force
    log_success "Frontend cleaned"

    log_success "All artifacts cleaned"
}

# Parse verbose flag
if [[ $1 == "-v" ]] || [[ $1 == "--verbose" ]]; then
    VERBOSE=1
    MODE=${2:-help}
elif [[ $2 == "-v" ]] || [[ $2 == "--verbose" ]]; then
    VERBOSE=1
fi

if [ $VERBOSE -eq 1 ]; then
    set -x
fi

# Main execution
case $MODE in
    dev)
        check_dependencies
        start_all
        ;;
    back)
        check_dependencies
        start_backend
        ;;
    front)
        check_dependencies
        start_frontend
        ;;
    build)
        check_dependencies
        build_all
        ;;
    build-back)
        check_dependencies
        build_backend
        ;;
    build-front)
        check_dependencies
        build_frontend
        ;;
    install)
        check_dependencies
        install_dependencies
        ;;
    clean)
        clean
        ;;
    help|"")
        show_help
        ;;
    *)
        log_error "Unknown command: $MODE"
        show_help
        exit 1
        ;;
esac
