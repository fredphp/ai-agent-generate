# AI Dev Agent - 启动指南

## 快速开始

### 方式一：使用快速启动脚本（推荐）

```bash
# 1. 设置 API Key
export GLM_API_KEY="your-api-key-here"

# 2. 运行脚本
./run.sh refactor main.go
./run.sh fix server/auth.go -- "Fix nil pointer dereference"
```

脚本会自动：
- 检查 Go 是否安装
- 检查 API Key 是否设置
- 编译项目
- 执行命令

---

### 方式二：手动编译运行

```bash
# 1. 设置 API Key
export GLM_API_KEY="your-api-key-here"

# 2. 编译
go build -o aidev ./cmd/aidev

# 3. 运行
./aidev refactor main.go
```

---

### 方式三：直接运行（开发模式）

```bash
# 无需编译，直接运行
go run ./cmd/aidev refactor main.go

# 带详细输出
go run ./cmd/aidev -V refactor main.go
```

---

## 环境要求

| 依赖 | 版本要求 | 说明 |
|------|---------|------|
| Go | >= 1.21 | 编译和运行 |
| GLM API Key | - | 从智谱 AI 获取 |

### 安装 Go

**macOS:**
```bash
brew install go
```

**Ubuntu/Debian:**
```bash
sudo apt update && sudo apt install -y golang-go
```

**Windows:**
从 https://go.dev/dl/ 下载安装包

**官方版本（推荐）:**
```bash
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### 获取 API Key

1. 访问 https://open.bigmodel.cn/
2. 注册/登录账号
3. 进入控制台获取 API Key
4. 设置环境变量：
   ```bash
   export GLM_API_KEY="your-api-key-here"
   ```

---

## CLI 命令

### 支持的命令

| 命令 | 说明 | 示例 |
|------|------|------|
| `refactor` | 重构代码 | `./aidev refactor main.go` |
| `fix` | 修复 Bug | `./aidev fix auth.go -- "Fix nil pointer"` |
| `generate` | 生成代码 | `./aidev generate api.go -- "Generate CRUD"` |
| `explain` | 解释代码 | `./aidev explain main.go` |
| `review` | 代码审查 | `./aidev review handler.go` |
| `test` | 生成测试 | `./aidev test utils.go` |

### CLI 选项

| 选项 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--api-key` | `-k` | 环境变量 | GLM API Key |
| `--model` | `-m` | glm-4-flash | 模型名称 |
| `--retries` | | 3 | 最大重试次数 |
| `--timeout` | | 2m | 请求超时 |
| `--verbose` | `-V` | false | 详细输出 |
| `--dry-run` | | false | 干运行（不写文件） |
| `--no-backup` | | false | 不创建备份 |
| `--workdir` | `-w` | 当前目录 | 工作目录 |
| `--help` | `-h` | | 显示帮助 |
| `--version` | `-v` | | 显示版本 |

---

## 运行本地项目示例

### 示例 1：重构本地项目文件

```bash
# 进入你的项目目录
cd /home/user/my-project

# 重构单个文件
/path/to/aidev refactor src/handler.go

# 重构并指定指令
/path/to/aidev refactor src/handler.go -- "Extract the validation logic into a separate function"

# 使用工作目录参数
/path/to/aidev -w /home/user/my-project refactor src/handler.go
```

### 示例 2：修复 Bug

```bash
# 修复已知 Bug
/path/to/aidev fix server/auth.go -- "Fix the nil pointer dereference in ValidateToken"

# 修复并验证构建
/path/to/aidev fix server/rbac.go -- "Fix the race condition in permission check"
```

### 示例 3：生成新代码

```bash
# 生成 API 处理器
/path/to/aidev generate api/v1/user.go -- "Generate REST API handlers for User model with CRUD operations"

# 生成服务层代码
/path/to/aidev generate internal/service/order.go -- "Generate OrderService with Create, Update, Delete, Get methods"

# 基于现有文件生成
/path/to/aidev generate internal/dto/user_dto.go -- "Generate DTO struct based on the User model"
```

### 示例 4：批量处理

```bash
# 重构多个文件
/path/to/aidev refactor src/handler.go src/service.go src/utils.go

# 修复整个目录
/path/to/aidev fix server/auth/*.go
```

### 示例 5：代码审查和解释

```bash
# 代码审查
/path/to/aidev review src/handler.go

# 解释复杂代码
/path/to/aidev explain internal/algorithm/sort.go

# 生成测试
/path/to/aidev test src/utils.go
```

---

## 工作流程

```
┌─────────────────────────────────────────────────────────────────────┐
│                        执行流程                                      │
└─────────────────────────────────────────────────────────────────────┘

$ ./aidev refactor server/handler.go -- "Add error handling"
        │
        ▼
┌─────────────────┐
│  1. 读取文件     │  读取 server/handler.go 内容
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  2. 构建 Prompt  │  组合代码 + 指令 → 发送给 AI
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  3. 调用 GLM API │  发送请求到 GLM API
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  4. 解析代码块   │  从 AI 响应中提取代码
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  5. 写入文件     │  自动备份原文件 → 写入新代码
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  6. 验证构建     │  运行 go build ./...
└────────┬────────┘
         │
    ┌────┴────┐
    │ 成功?   │
    └────┬────┘
         │
    ┌────┴────┐
    │         │
   否        是
    │         │
    ▼         ▼
┌────────┐ ┌────────┐
│ 自动   │ │ 完成   │
│ 重试   │ └────────┘
└────────┘
```

---

## 输出示例

```bash
$ ./aidev refactor server/handler.go -V

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Command:     refactor
Files:       [server/handler.go]
Instruction: Add error handling
Model:       glm-4-flash
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[INFO] Starting refactor operation on 1 file(s)
[INFO] Attempt 1/3
[INFO] LLM response received (2341 chars)
[INFO] Parsed 1 code block(s)
[INFO] Wrote: server/handler.go
[INFO] Build verification passed

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
                    EXECUTION RESULT
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Operation completed successfully!

Files changed:
  📝 server/handler.go

Attempts: 1
Duration: 5.2s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

---

## 自动重试机制

当构建验证失败时，系统会自动重试：

1. **第 1 次失败**：附加构建错误信息到指令中，重新生成
2. **第 2 次失败**：继续重试
3. **第 3 次失败**：返回错误，恢复备份

```bash
$ ./aidev fix server/auth.go

[INFO] Attempt 1/3
[ERROR] Build verification failed: undefined: x
[INFO] Attempt 2/3
[ERROR] Build verification failed: syntax error
[INFO] Attempt 3/3
[SUCCESS] Build verification passed
```

---

## 备份机制

每次修改文件前自动创建备份：

```bash
# 原文件
server/handler.go

# 备份文件
server/handler.go.bak.20250225-120000
```

恢复备份：
```bash
cp server/handler.go.bak.20250225-120000 server/handler.go
```

---

## 常见问题

### Q: API Key 如何获取？

访问 https://open.bigmodel.cn/ 注册并获取 API Key。

### Q: 支持哪些模型？

- `glm-4-flash`（默认，速度快）
- `glm-4`（质量高）
- `glm-4-plus`（最强）

### Q: 如何调试？

使用 `-V` 或 `--verbose` 参数：
```bash
./aidev -V refactor main.go
```

### Q: 如何测试而不修改文件？

使用 `--dry-run` 参数：
```bash
./aidev --dry-run refactor main.go
```

### Q: 支持哪些编程语言？

理论上支持所有编程语言，针对 Go、TypeScript、Python 有优化。

### Q: 构建验证支持其他语言吗？

目前仅支持 Go 项目的 `go build` 验证。其他语言可通过 `--no-build` 跳过。

---

## 项目结构

```
ai-dev-agent/
├── run.sh                    # 快速启动脚本
├── start.md                  # 本文档
├── go.mod                    # Go 模块定义
├── cmd/aidev/main.go         # CLI 入口
└── service/                  # 服务层
    ├── llm/                  # GLM API 客户端
    ├── filesystem/           # 文件操作
    ├── prompt/               # Prompt 构建
    ├── executor/             # 命令执行
    └── orchestrator/         # 工作流编排
```

---

## 更多信息

- GitHub: https://github.com/fredphp/ai-agent-generate
- GLM API 文档: https://open.bigmodel.cn/dev/api
