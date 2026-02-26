#!/bin/bash
# ============================================================================
# AI Dev Agent - Quick Start Script
# ============================================================================
# 
# 用法:
#   ./quick-start.sh refactor main.go
#   ./quick-start.sh fix server/auth.go -- "Fix nil pointer"
#   ./quick-start.sh generate api/user.go -- "Generate CRUD handlers"
#
# 环境变量:
#   GLM_API_KEY   - GLM API Key (必需)
#   GLM_MODEL     - 模型名称 (可选, 默认: glm-4-flash)
#
# ============================================================================

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印函数
info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# ============================================================================
# Step 1: 检查 Go 环境
# ============================================================================

info "检查 Go 环境..."

if ! command -v go &> /dev/null; then
    error "Go 未安装!

请先安装 Go:
  macOS:     brew install go
  Ubuntu:    sudo apt install -y golang-go
  Windows:   从 https://go.dev/dl/ 下载安装包

安装后运行: go version"
fi

GO_VERSION=$(go version | grep -oP 'go\d+\.\d+' | head -1)
info "检测到 Go 版本: $GO_VERSION"

# ============================================================================
# Step 2: 检查 API Key
# ============================================================================

info "检查 API Key..."

if [ -z "$GLM_API_KEY" ] && [ -z "$ZHIPUAI_API_KEY" ]; then
    error "API Key 未设置!

请设置环境变量:
  export GLM_API_KEY='your-api-key-here'

获取 API Key:
  1. 访问 https://open.bigmodel.cn/
  2. 注册/登录账号
  3. 在控制台获取 API Key"
fi

API_KEY="${GLM_API_KEY:-$ZHIPUAI_API_KEY}"
info "API Key 已设置 (${API_KEY:0:8}...)"

# ============================================================================
# Step 3: 编译项目
# ============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

BINARY="./aidev"

if [ ! -f "$BINARY" ] || [ "cmd/aidev/main.go" -nt "$BINARY" ]; then
    info "编译项目..."
    go build -o "$BINARY" ./cmd/aidev
    success "编译完成: $BINARY"
else
    info "使用已编译的二进制文件"
fi

# ============================================================================
# Step 4: 运行
# ============================================================================

if [ $# -eq 0 ]; then
    info "显示帮助信息..."
    "$BINARY" --help
    exit 0
fi

info "执行命令: aidev $@"
echo ""

export GLM_API_KEY="$API_KEY"
"$BINARY" "$@"

echo ""
success "执行完成!"
