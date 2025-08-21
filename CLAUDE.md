# CLAUDE.md

此文件为在此代码仓库中使用 Claude Code (claude.ai/code) 提供指导。

## 项目概述

这是一个 Linux MCP（模型上下文协议）服务管理器 - 通过多种协议管理 Linux 系统服务的服务器。支持 systemd、System V init 服务和 Docker 容器。

### 支持的协议
- **MCP stdio**: 通过标准输入输出的原生 MCP 协议（用于 AI 模型直接集成）
- **HTTP REST API**: 传统的 RESTful HTTP API
- **MCP over HTTP (SSE)**: 通过 Server-Sent Events 的 MCP 协议
- **MCP Streamable HTTP**: 支持双向流式传输的 MCP 协议

## 构建和运行命令

### 基本构建命令
```bash
# 构建主服务器
go build -o mcp-server cmd/server/main.go

# 或从根目录直接运行（使用调用 cmd/server/main.go 的包装器）
go run main.go [选项]

# 安装依赖
go mod tidy
```

### 服务器模式
```bash
# MCP stdio 模式（默认）- 用于 AI 模型直接集成
./mcp-server -mode=mcp

# HTTP REST API 模式
./mcp-server -mode=http -config=config.yaml

# MCP over HTTP (SSE) 模式
./mcp-server -mode=mcp-http

# MCP Streamable HTTP 模式
./mcp-server -mode=mcp-streamable

# 使用简化命令行选项
./mcp-server -mcp-http                # 等同于 -mode=mcp-http
./mcp-server -mcp-streamable         # 等同于 -mode=mcp-streamable
./mcp-server -http                   # 等同于 -mode=http

# 显示帮助
./mcp-server -help
```

### 测试命令
```bash
# 运行所有测试
go test ./...

# 运行测试并显示详细输出
go test -v ./...

# 运行特定包的测试
go test -v ./internal/managers
go test -v ./pkg/types
```

## 架构概述

### 核心架构
项目采用清晰的分层架构：

1. **入口点**: `cmd/server/main.go` - 主应用程序入口，配置和服务器模式选择
2. **传输层**: 
   - `internal/server/http.go` - HTTP REST API 服务器
   - `internal/server/mcp_http.go` - MCP over HTTP (SSE) 服务器
   - `internal/server/mcp_streamable.go` - MCP Streamable HTTP 服务器
   - `internal/mcp/server.go` - 原生 MCP stdio 协议服务器
3. **服务层**: `internal/managers/` - 不同后端的服务管理实现
4. **类型定义**: `pkg/types/` - 核心数据结构和接口

### 关键组件

**服务管理器** (`internal/managers/`):
- `systemd.go` - systemd 服务管理
- `sysv.go` - System V init 服务管理  
- `docker.go` - Docker 容器管理
- 都实现了 `types.ServiceManager` 接口

**MCP 集成** (`internal/mcp/`):
- `server.go` - MCP 协议实现，为 AI 模型提供工具和提示词
- 提供工具：list_services, get_service_status, start_service, stop_service, restart_service, enable_service, disable_service, get_docker_logs
- 提供提示词：service_management_help, service_troubleshooting

**配置管理** (`internal/config/`):
- `config.go` - 基于 YAML 的配置管理，支持环境变量覆盖

### 服务类型检测
应用程序在启动时自动检测可用的服务管理器：
- 检查 systemd 可用性
- 检查 System V init 脚本
- 检查 Docker 守护进程
- 只初始化可用的管理器

### 多协议支持
- **MCP stdio**: 通过 stdin/stdout JSON-RPC 与 Claude 等 AI 模型集成
- **HTTP REST API**: 传统的 REST 端点用于服务管理
- **MCP over HTTP (SSE)**: 通过 HTTP 和服务器发送事件的 MCP 协议
- **MCP Streamable HTTP**: 支持双向流式传输的 MCP 协议，适用于需要长连接或实时交互的场景

## 开发说明

### 添加新的服务管理器
1. 在 `internal/managers/` 中实现 `types.ServiceManager` 接口
2. 添加检测函数（如 `IsNewManagerAvailable()`）
3. 在服务器构造函数中初始化
4. 在 `pkg/types/service.go` 中添加对应的服务类型

### 添加 MCP 工具
1. 在 `internal/mcp/server.go` 的 `handleListTools()` 中定义工具模式
2. 在 `handleCallTool()` 中实现工具逻辑
3. 添加任何必需的辅助函数

### 配置
- 使用 YAML 配置文件，支持环境变量覆盖
- 默认配置路径：`config.yaml`
- 环境变量：`MCP_HOST`, `MCP_PORT`, `MCP_LOG_LEVEL`, `MCP_LOG_FORMAT`

## 测试结构

测试文件与源文件同目录，使用 `*_test.go` 命名：
- 每个管理器的单元测试在 `internal/managers/*_test.go`
- HTTP 服务器测试在 `internal/server/http_test.go`
- 类型测试在 `pkg/types/*_test.go`
- 工具测试在 `pkg/utils/utils_test.go`

## 文档更新规则

**重要：每当添加新功能、修改现有功能或更改项目架构时，必须同时更新以下文档：**

1. **CLAUDE.md**（本文件）- 更新开发指导信息：
   - 新增的命令行选项和服务器模式
   - 架构变化（新增的文件、组件、接口）
   - 构建和测试命令的变化
   - 新的开发流程或最佳实践

2. **README.md** - 更新用户使用文档：
   - 功能特性列表
   - 安装和使用方法
   - 命令行选项和示例
   - API 端点（如有变化）
   - 配置选项

### 文档更新检查清单
当进行代码更改时，请检查是否需要更新：
- [ ] 新增或修改了服务器模式？ → 更新两个文档中的服务器模式部分
- [ ] 新增或修改了命令行选项？ → 更新两个文档中的命令行使用部分  
- [ ] 新增了新的协议支持？ → 更新协议支持说明
- [ ] 修改了架构或添加了新文件？ → 更新 CLAUDE.md 中的架构说明
- [ ] 新增了 API 端点？ → 更新 README.md 中的 API 文档
- [ ] 修改了配置选项？ → 更新配置相关文档

### 文档同步的重要性
保持文档与代码同步确保：
- 新的开发者能快速理解项目结构
- 用户能正确使用所有功能
- 项目维护的连续性和专业性