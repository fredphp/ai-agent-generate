#!/bin/bash
# ============================================================================
# AI Dev Agent - Quick Start Script
# ============================================================================
#
# Usage:
#   ./run.sh <command> <files...> [-- instruction]
#
# Examples:
#   ./run.sh refactor main.go
#   ./run.sh fix server/auth.go -- "Fix nil pointer"
#   ./run.sh generate api/user.go -- "Generate CRUD handlers"
#
# Environment:
#   GLM_API_KEY - Your GLM API key (required)
#
# ============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_NAME="aidev"

# ============================================================================
# Helper Functions
# ============================================================================

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_banner() {
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║${NC}           AI Dev Agent - Quick Start Script              ${BLUE}║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# ============================================================================
# Check Prerequisites
# ============================================================================

check_go() {
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed!"
        echo ""
        echo "Please install Go from: https://go.dev/dl/"
        echo ""
        echo "  macOS:   brew install go"
        echo "  Ubuntu:  sudo apt install golang-go"
        echo "  Windows: Download from https://go.dev/dl/"
        echo ""
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    print_success "Go version: $GO_VERSION"
}

check_api_key() {
    if [ -z "$GLM_API_KEY" ] && [ -z "$ZHIPUAI_API_KEY" ]; then
        print_error "API key not found!"
        echo ""
        echo "Please set your GLM API key:"
        echo ""
        echo "  export GLM_API_KEY=\"your-api-key-here\""
        echo ""
        echo "Get your API key from: https://open.bigmodel.cn/"
        echo ""
        exit 1
    fi
    
    if [ -n "$GLM_API_KEY" ]; then
        print_success "GLM_API_KEY is set (${GLM_API_KEY:0:8}...)"
    else
        print_success "ZHIPUAI_API_KEY is set (${ZHIPUAI_API_KEY:0:8}...)"
    fi
}

# ============================================================================
# Build
# ============================================================================

build_binary() {
    print_info "Building binary..."
    
    cd "$SCRIPT_DIR"
    
    if [ -f "$BINARY_NAME" ]; then
        print_info "Binary already exists, rebuilding..."
    fi
    
    go build -o "$BINARY_NAME" ./cmd/aidev
    
    if [ -f "$BINARY_NAME" ]; then
        print_success "Build successful: ./$BINARY_NAME"
    else
        print_error "Build failed!"
        exit 1
    fi
}

# ============================================================================
# Run
# ============================================================================

run() {
    if [ $# -eq 0 ]; then
        print_error "No command specified!"
        echo ""
        echo "Usage: $0 <command> <files...> [-- instruction]"
        echo ""
        echo "Commands:"
        echo "  refactor    Refactor code"
        echo "  fix         Fix bugs"
        echo "  generate    Generate code"
        echo "  explain     Explain code"
        echo "  review      Review code"
        echo "  test        Generate tests"
        echo ""
        echo "Examples:"
        echo "  $0 refactor main.go"
        echo "  $0 fix server/auth.go -- \"Fix nil pointer\""
        echo ""
        exit 1
    fi
    
    print_info "Running: ./aidev $*"
    echo ""
    
    "./$BINARY_NAME" "$@"
    
    local exit_code=$?
    
    echo ""
    if [ $exit_code -eq 0 ]; then
        print_success "Command completed successfully!"
    else
        print_error "Command failed with exit code: $exit_code"
    fi
    
    exit $exit_code
}

# ============================================================================
# Main
# ============================================================================

main() {
    print_banner
    
    # Check prerequisites
    print_info "Checking prerequisites..."
    check_go
    check_api_key
    echo ""
    
    # Build
    build_binary
    echo ""
    
    # Run
    run "$@"
}

# Entry point
main "$@"
