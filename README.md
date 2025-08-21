# Linux 服务管理器

一个全面的Linux系统服务管理服务器，支持多种服务管理方式，包括systemd、System V init和Docker容器。

**现已支持MCP（模型上下文协议），可与Claude等AI模型集成！**

## 功能特性

- **多平台支持**: systemd、System V init、Docker
- **多协议支持**: 支持4种不同的协议接口
  - MCP stdio（原生AI模型集成）
  - HTTP REST API（传统RESTful接口）
  - MCP over HTTP SSE（服务器发送事件）
  - MCP Streamable HTTP（双向流式传输）
- **MCP集成**: 通过模型上下文协议原生支持AI模型
- **自动检测**: 自动检测可用的服务管理器
- **Docker集成**: 将Docker容器作为服务管理
- **AI友好工具**: 为AI模型交互预定义工具和提示词
- **完善的日志**: 支持多种格式的可配置日志
- **配置管理**: 基于YAML的配置文件，支持环境变量覆盖
- **流式传输**: 支持实时双向通信和长连接

## 支持的服务类型

### 1. systemd服务
- 启动、停止、重启服务
- 启用/禁用服务开机自启
- 获取详细的服务状态和日志
- 列出所有systemd服务

### 2. System V init服务
- 通过`/etc/init.d`脚本控制服务
- 支持`chkconfig`和`update-rc.d`
- 从LSB头部提取服务信息
- 兼容传统Linux发行版

### 3. Docker容器
- 启动、停止、重启容器
- 列出所有容器及其状态
- 获取容器日志和统计信息
- 创建和删除容器
- 管理容器重启策略

## 安装方法

```bash
# 克隆仓库
git clone <repository-url>
cd mcp_srv_mgr

# 构建应用程序
go build -o mcp-server cmd/server/main.go

# MCP stdio模式（默认，AI模型直接集成）
./mcp-server

# HTTP REST API模式
./mcp-server -mode=http

# MCP over HTTP (SSE) 模式
./mcp-server -mode=mcp-http

# MCP Streamable HTTP 模式
./mcp-server -mode=mcp-streamable

# 使用配置文件
./mcp-server -config=config.yaml
```

## MCP（模型上下文协议）使用方法

此服务器可通过MCP协议与Claude等AI模型一起使用。

### Claude Desktop集成

1. **构建MCP服务器：**
```bash
go build -o mcp-server cmd/server/main.go
```

2. **添加到Claude Desktop配置：**

编辑Claude Desktop配置文件（在macOS上通常位于`~/Library/Application Support/Claude/claude_desktop_config.json`）：

```json
{
  "mcpServers": {
    "linux-service-manager": {
      "command": "/完整路径/到/你的/mcp-server",
      "args": ["-mode=mcp"]
    }
  }
}
```

3. **重启Claude Desktop** - 服务管理器工具将在您的Claude对话中可用。

### 可用的MCP工具

通过MCP连接时，AI模型可以使用以下工具：

- **`list_services`** - 列出所有可用服务（支持类型筛选）
- **`get_service_status`** - 获取特定服务的详细状态
- **`start_service`** - 启动服务
- **`stop_service`** - 停止服务
- **`restart_service`** - 重启服务
- **`enable_service`** - 启用服务自动启动
- **`disable_service`** - 禁用服务自动启动
- **`get_docker_logs`** - 从Docker容器获取日志

### 可用的MCP提示词

- **`service_management_help`** - 获取Linux服务管理的全面帮助
- **`service_troubleshooting`** - 获取服务问题的故障排除指导

### MCP使用示例

配置好Claude Desktop后，您可以提出这样的问题：

- "显示所有运行中的systemd服务"
- "nginx服务的状态如何？"
- "启动名为web-server的docker容器"
- "帮我排查mysql服务无法启动的问题"
- "获取我的app容器的最后50行日志"

AI模型将使用适当的工具来管理您的服务并提供有用的响应。

## 配置

### 配置文件 (config.yaml)

```yaml
server:
  host: "127.0.0.1"
  port: 8080

log:
  level: "info"      # debug, info, warn, error
  format: "json"     # json, text
  output: "stdout"   # stdout, stderr
```

### 环境变量

- `MCP_HOST`: 服务器主机（默认：127.0.0.1）
- `MCP_PORT`: 服务器端口（默认：8080）
- `MCP_LOG_LEVEL`: 日志级别（默认：info）
- `MCP_LOG_FORMAT`: 日志格式（默认：json）

## API端点

### 服务管理

#### 列出所有服务
```http
GET /services
GET /services?type=systemd
GET /services?type=sysv
GET /services?type=docker
```

#### 获取服务状态
```http
GET /services/{name}/status
GET /services/{name}/status?type=systemd
```

#### 服务操作
```http
POST /services/{name}/start
POST /services/{name}/stop
POST /services/{name}/restart
POST /services/{name}/enable
POST /services/{name}/disable
```

#### 通用服务操作
```http
POST /services/action
Content-Type: application/json

{
  "name": "nginx",
  "type": "systemd",
  "action": "start"
}
```

### Docker专用端点

#### 获取容器日志
```http
GET /docker/{name}/logs?lines=100
```

#### 获取容器统计信息
```http
GET /docker/{name}/stats
```

#### 删除容器
```http
DELETE /docker/{name}/remove?force=true
```

#### 创建容器
```http
POST /docker/create
Content-Type: application/json

{
  "image_name": "nginx:latest",
  "container_name": "my-nginx",
  "options": ["-p", "80:80"]
}
```

### 系统端点

#### 健康检查
```http
GET /health
```

#### 服务器信息
```http
GET /info
```

## 使用示例

### 使用curl

```bash
# 列出所有服务
curl http://localhost:8080/services

# 获取nginx服务状态
curl http://localhost:8080/services/nginx/status

# 启动nginx服务
curl -X POST http://localhost:8080/services/nginx/start

# 停止docker容器
curl -X POST http://localhost:8080/services/my-container/stop?type=docker

# 获取docker容器日志
curl http://localhost:8080/docker/my-container/logs?lines=50

# 健康检查
curl http://localhost:8080/health
```

### 响应格式

#### 服务列表响应
```json
{
  "success": true,
  "message": "服务列表获取成功",
  "services": [
    {
      "name": "nginx",
      "type": "systemd",
      "status": "active",
      "description": "nginx HTTP和反向代理服务器",
      "pid": 1234,
      "uptime": "2h30m15s",
      "last_changed": "2023-01-01T10:00:00Z"
    }
  ]
}
```

#### 服务状态响应
```json
{
  "success": true,
  "message": "服务状态获取成功",
  "service": {
    "name": "nginx",
    "type": "systemd",
    "status": "active",
    "description": "nginx HTTP和反向代理服务器",
    "pid": 1234,
    "uptime": "2h30m15s",
    "last_changed": "2023-01-01T10:00:00Z"
  }
}
```

## 命令行选项

```bash
# 显示帮助
./mcp-server -help

# MCP stdio 模式（默认，用于AI模型集成）
./mcp-server
./mcp-server -mode=mcp

# HTTP REST API 模式
./mcp-server -http
./mcp-server -mode=http

# MCP over HTTP (SSE) 模式
./mcp-server -mcp-http
./mcp-server -mode=mcp-http

# MCP Streamable HTTP 模式
./mcp-server -mcp-streamable
./mcp-server -mode=mcp-streamable

# 使用自定义配置文件
./mcp-server -config=/path/to/config.yaml -mode=http
```

### 协议说明

1. **MCP stdio**: 通过标准输入输出与AI模型直接通信，最适合Claude Desktop等集成场景
2. **HTTP REST API**: 传统的RESTful API，适合Web应用和脚本调用
3. **MCP over HTTP (SSE)**: 使用Server-Sent Events的MCP协议，适合需要推送通知的场景
4. **MCP Streamable HTTP**: 支持双向流式传输，适合需要实时交互或长连接的应用

## 系统要求

- Go 1.21或更高版本
- Linux操作系统
- 某些服务操作需要root权限
- systemd（可选，用于systemd服务）
- Docker（可选，用于容器管理）
- Claude Desktop（用于与AI模型的MCP集成）

## 安全注意事项

- 服务器需要适当的权限来管理服务
- 在可能的情况下考虑使用受限权限运行
- 使用防火墙规则限制对API端点的访问
- 在生产环境中启用身份验证/授权

## 错误处理

服务器提供适当HTTP状态码的全面错误处理：

- `200 OK`: 操作成功
- `400 Bad Request`: 无效的请求参数
- `404 Not Found`: 服务未找到
- `500 Internal Server Error`: 操作失败
- `503 Service Unavailable`: 服务管理器不可用

## 日志记录

所有操作都会记录日志，支持可配置的级别和格式：

- **级别**: debug、info、warn、error
- **格式**: json、text
- **输出**: stdout、stderr

## 故障排除

### MCP服务器无法启动
- 检查Go环境和依赖
- 验证文件权限
- 查看日志输出（使用`-log-level debug`）

### Claude Desktop无法连接
- 验证配置文件路径和格式
- 确认MCP服务器可执行权限
- 重启Claude Desktop

### 服务操作失败
- 检查服务管理器是否可用（systemd/docker等）
- 验证用户权限
- 查看系统日志

## 开发说明

### 项目结构
```
mcp_srv_mgr/
├── main.go              # 主程序入口
├── types.go             # 数据结构定义
├── server.go            # HTTP服务器和路由
├── mcp_server.go        # MCP协议服务器
├── mcp_types.go         # MCP协议数据结构
├── systemd.go           # systemd服务管理
├── sysv.go             # System V init服务管理
├── docker.go           # Docker容器管理
├── config.go           # 配置文件处理
├── utils.go            # 工具函数
├── go.mod              # Go模块定义
├── config.yaml.example # 配置文件示例
├── README_CN.md        # 中文文档
└── MCP_USAGE.md        # MCP使用指南
```

### 扩展功能

#### 添加新的MCP工具
1. 在`mcp_server.go`的`handleListTools`中添加工具定义
2. 在`handleCallTool`中添加工具处理逻辑
3. 实现具体的工具函数

#### 添加新的MCP提示词
1. 在`handleListPrompts`中添加提示词定义
2. 在`handleGetPrompt`中添加提示词处理逻辑
3. 实现提示词内容生成函数

## 许可证

本项目采用MIT许可证。