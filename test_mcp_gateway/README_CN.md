# mcp_srv_mgr 的 Unla Gateway 集成测试

本目录包含了将 `mcp_srv_mgr` 服务与 Unla MCP Gateway 连接的全面集成测试。测试验证了所有支持的协议（HTTP API、MCP stdio、MCP over HTTP SSE、MCP Streamable HTTP）通过网关的正确工作。

## 概述

`mcp_srv_mgr` 是一个 Linux 服务管理系统，提供多种协议接口：
- **HTTP REST API**：传统的 REST 端点，用于服务管理
- **MCP stdio**：通过 stdin/stdout 的原生 MCP 协议，用于 AI 模型集成
- **MCP over HTTP (SSE)**：基于 HTTP 和服务器发送事件的 MCP 协议
- **MCP Streamable HTTP**：支持双向流式传输的 MCP 协议

Unla Gateway 充当代理，可以集成这些服务并通过统一的 MCP 接口向 AI 模型提供服务。

## 架构图

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│                 │    │                 │    │                 │
│   AI 模型       │◄───┤  Unla Gateway   │◄───┤  mcp_srv_mgr    │
│  (Claude 等)    │    │                 │    │                 │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │                        │
                              │                        ├─ HTTP API (8080)
                              │                        ├─ MCP stdio
                              │                        ├─ MCP HTTP SSE (8082)
                              │                        └─ MCP Streamable (8083)
                              │
                        ┌─────────────────┐
                        │                 │
                        │  MySQL + Redis  │
                        │                 │
                        └─────────────────┘
```

## 文件结构

```
test_mcp_gateway/
├── README_CN.md                       # 中文文档（本文件）
├── README.md                          # 英文文档
├── unla-config.yaml                   # Unla Gateway 配置文件
├── config.yaml                        # mcp_srv_mgr HTTP 模式配置
├── config-mcp-http.yaml              # mcp_srv_mgr MCP HTTP 模式配置
├── config-mcp-streamable.yaml        # mcp_srv_mgr MCP Streamable 模式配置
├── docker-compose.yml                # MySQL 和 Redis 容器
├── init.sql                          # 数据库初始化脚本
├── test_http_api.sh                  # HTTP API 集成测试
├── test_mcp_stdio.sh                 # MCP stdio 协议测试
├── test_mcp_http_sse.sh              # MCP over HTTP SSE 测试
├── test_mcp_streamable.sh            # MCP Streamable HTTP 测试
├── test_mysql_integration.sh         # 数据库集成测试
├── run_all_tests.sh                  # 主测试运行器
└── test_readme.md                    # 原始需求文档
```

## 系统要求

### 必需软件
- **Go**（用于构建 mcp-server）
- **MySQL 客户端**（用于数据库测试）
- **Redis CLI**（用于缓存测试）
- **curl**（用于 HTTP 测试）
- **jq**（用于 JSON 处理，可选）
- **Docker & Docker Compose**（用于数据库容器）

### 必需服务
- **Unla Gateway 二进制文件**：当前目录下应有 `mcp-gateway`
- **MySQL 数据库**：localhost:3311，凭据 root/nov24feb11
- **Redis 缓存**：localhost:6379（可选，用于缓存测试）

## 快速开始

### 1. 构建 mcp_srv_mgr
```bash
# 在主项目目录下
go build -o mcp-server cmd/server/main.go
```

### 2. 启动数据库服务
```bash
# 启动 MySQL 和 Redis 容器
docker-compose -f test_mcp_gateway/docker-compose.yml up -d
```

### 3. 运行所有测试
```bash
# 运行完整测试套件
./test_mcp_gateway/run_all_tests.sh
```

### 4. 启动 Unla Gateway（测试通过后）
```bash
# 使用生成的配置启动网关
./mcp-gateway --config test_mcp_gateway/unla-config.yaml
```

## 各项测试脚本

### HTTP API 集成测试 (`test_http_api.sh`)
测试 HTTP REST API 端点的直接访问和通过 Unla Gateway 的访问：
- 健康检查
- 服务列表
- 服务状态查询
- 服务操作（启动/停止/重启）
- Docker 操作
- 网关代理功能

**使用方法：**
```bash
./test_mcp_gateway/test_http_api.sh
```

### MCP stdio 协议测试 (`test_mcp_stdio.sh`)
测试通过 stdin/stdout 的原生 MCP 协议：
- MCP 会话初始化
- 工具列表和执行
- 提示管理
- 网关 stdio 代理

**使用方法：**
```bash
./test_mcp_gateway/test_mcp_stdio.sh
```

### MCP HTTP SSE 测试 (`test_mcp_http_sse.sh`)
测试基于 HTTP 和服务器发送事件的 MCP：
- 基于 HTTP 的 MCP 通信
- SSE 流式传输
- 通过 HTTP 的工具执行
- 网关 HTTP 代理

**使用方法：**
```bash
./test_mcp_gateway/test_mcp_http_sse.sh
```

### MCP Streamable HTTP 测试 (`test_mcp_streamable.sh`)
测试双向流式传输 MCP 协议：
- WebSocket 类似的通信
- 流式工具执行
- 长时间运行的操作
- 网关流式代理

**使用方法：**
```bash
./test_mcp_gateway/test_mcp_streamable.sh
```

### MySQL 集成测试 (`test_mysql_integration.sh`)
测试数据库连接和操作：
- MySQL 连接和表创建
- CRUD 操作
- JSON 数据处理
- Redis 缓存操作
- 事务支持

**使用方法：**
```bash
./test_mcp_gateway/test_mysql_integration.sh
```

## 配置文件说明

### `unla-config.yaml`
Unla Gateway 的主配置文件，定义了：
- **数据库设置**：会话管理的 MySQL 连接
- **Redis 设置**：缓存配置
- **服务器设置**：网关服务器配置
- **集成定义**：如何连接到 mcp_srv_mgr 服务
- **工具映射**：REST API 到 MCP 工具的转换

主要部分：
- `database`：MySQL 配置（主机：127.0.0.1:3311）
- `redis`：Redis 配置（主机：127.0.0.1:6379）
- `integrations`：服务集成定义
  - `mcp_srv_mgr_http`：HTTP API 集成
  - `mcp_srv_mgr_stdio`：MCP stdio 集成
  - `mcp_srv_mgr_http_sse`：MCP over HTTP 集成
  - `mcp_srv_mgr_streamable`：MCP Streamable 集成

### 服务配置文件
- `config.yaml`：HTTP 模式配置（端口 8080）
- `config-mcp-http.yaml`：MCP HTTP 模式配置（端口 8082）
- `config-mcp-streamable.yaml`：MCP Streamable 模式配置（端口 8083）

## 数据库架构

MySQL 数据库 (`unla_gateway`) 包含以下表：

### `unla_sessions`
存储活动会话及其状态：
```sql
id VARCHAR(255) PRIMARY KEY           -- 会话标识符
user_id VARCHAR(255)                  -- 用户标识符
integration_name VARCHAR(255)         -- 使用的集成名称
data TEXT                            -- 会话数据（JSON）
created_at TIMESTAMP                 -- 创建时间
updated_at TIMESTAMP                 -- 最后更新时间
expires_at TIMESTAMP                 -- 过期时间
```

### `unla_configurations`
存储网关和集成配置：
```sql
id INT AUTO_INCREMENT PRIMARY KEY    -- 配置 ID
name VARCHAR(255) UNIQUE             -- 配置名称
config JSON                          -- 配置数据
version INT                          -- 配置版本
created_at TIMESTAMP                 -- 创建时间
updated_at TIMESTAMP                 -- 最后更新时间
```

### `unla_metrics`
存储操作指标：
```sql
id INT AUTO_INCREMENT PRIMARY KEY    -- 指标 ID
integration_name VARCHAR(255)        -- 集成名称
tool_name VARCHAR(255)              -- 工具名称
execution_time_ms INT               -- 执行时间
status VARCHAR(50)                  -- 状态（成功/错误）
error_message TEXT                  -- 错误详情（如有）
timestamp TIMESTAMP                 -- 指标时间戳
```

## 支持的工具

网关从 mcp_srv_mgr 公开以下 MCP 工具：

### 服务管理工具
- `list_services`：列出所有可用服务
- `get_service_status`：获取特定服务的状态
- `start_service`：启动服务
- `stop_service`：停止服务
- `restart_service`：重启服务
- `enable_service`：启用开机启动
- `disable_service`：禁用开机启动

### Docker 专用工具
- `get_docker_logs`：获取容器日志

### 支持的服务类型
- **systemd**：现代 Linux 服务管理
- **sysv**：传统 System V init 服务
- **docker**：Docker 容器管理

## 测试策略

### 测试阶段
1. **前置条件检查**：验证所需工具和依赖项
2. **环境设置**：启动数据库容器并准备环境
3. **数据库集成**：测试 MySQL 和 Redis 连接及操作
4. **直接协议测试**：直接针对 mcp_srv_mgr 测试每个协议
5. **网关集成测试**：通过 Unla Gateway 代理测试协议
6. **清理**：停止服务并清理测试数据

### 测试覆盖
- ✅ **连接性测试**：验证所有服务可达
- ✅ **协议测试**：测试每个 MCP 协议变体
- ✅ **工具执行**：验证所有工具正确工作
- ✅ **错误处理**：测试错误条件和恢复
- ✅ **性能测试**：基本性能和超时测试
- ✅ **数据持久化**：数据库操作和会话管理
- ✅ **网关代理**：端到端网关功能

### 测试结果
每个测试脚本提供：
- **通过/失败状态**：测试结果的清晰指示
- **详细输出**：特定错误消息和调试信息
- **性能指标**：执行时间和成功率
- **清理**：测试数据的自动清理

## 故障排除

### 常见问题

**1. MySQL 连接失败**
```bash
# 检查 MySQL 是否运行
docker-compose -f test_mcp_gateway/docker-compose.yml ps mysql

# 查看 MySQL 日志
docker-compose -f test_mcp_gateway/docker-compose.yml logs mysql

# 重启 MySQL
docker-compose -f test_mcp_gateway/docker-compose.yml restart mysql
```

**2. mcp-server 构建失败**
```bash
# 确保 Go 模块是最新的
go mod tidy

# 使用详细输出构建
go build -v -o mcp-server cmd/server/main.go
```

**3. 端口冲突**
```bash
# 检查端口占用情况
lsof -i :8080  # HTTP API
lsof -i :8081  # Unla Gateway
lsof -i :8082  # MCP HTTP
lsof -i :8083  # MCP Streamable
lsof -i :3311  # MySQL
lsof -i :6379  # Redis
```

**4. 网关连接失败**
```bash
# 检查 Unla Gateway 是否运行
curl -f http://127.0.0.1:8081/health

# 查看网关日志
./mcp-gateway --config test_mcp_gateway/unla-config.yaml --log-level debug
```

### 调试模式
运行单独的测试并获得更详细的输出：
```bash
# 在脚本中启用调试输出
export DEBUG=1
./test_mcp_gateway/test_http_api.sh

# 使用 bash 调试运行
bash -x ./test_mcp_gateway/test_http_api.sh
```

## 与 AI 模型的集成

所有测试通过且 Unla Gateway 运行后，AI 模型可以连接到：
- **网关 URL**：`http://127.0.0.1:8081`
- **协议**：MCP over HTTP
- **可用工具**：所有 mcp_srv_mgr 服务管理工具

MCP 客户端连接示例：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list",
  "params": {}
}
```

网关将把这个请求代理到相应的 mcp_srv_mgr 实例并返回统一的响应。

## 安全考虑

### 当前配置（测试）
- **身份验证**：测试时已禁用
- **网络**：仅限本地主机（127.0.0.1）
- **数据库**：默认密码（生产环境中需更改）

### 生产环境建议
- **启用身份验证**：设置 `security.enable_auth: true`
- **使用 HTTPS**：配置 TLS 证书
- **安全数据库**：使用强密码和连接加密
- **网络安全**：使用防火墙规则和 VPN
- **监控**：启用指标和日志记录

## 性能考虑

### 资源使用
- **内存**：每个 mcp_srv_mgr 实例约 50MB
- **CPU**：空闲时最小，根据请求量扩展
- **数据库**：针对并发会话优化
- **网络**：启用 HTTP keep-alive

### 扩展
- **水平扩展**：网关后面的多个 mcp_srv_mgr 实例
- **数据库**：带有只读副本的 MySQL
- **缓存**：Redis 用于会话和响应缓存
- **负载均衡**：多个 Unla Gateway 实例

## 参与贡献

要添加新测试或修改现有测试：

1. **遵循现有的测试结构**
2. **使用标准状态报告函数**
3. **包含直接测试和网关测试**
4. **添加适当的清理程序**
5. **更新此文档**

### 测试模板
```bash
#!/bin/bash
# 测试描述在这里

# 标准设置
print_test_status() { ... }
make_request() { ... }

# 测试实现
echo "测试功能 X..."
if test_condition; then
    print_test_status "功能 X" "PASS"
else
    print_test_status "功能 X" "FAIL"
fi

# 清理
cleanup() { ... }
trap cleanup EXIT
```

## 使用说明

### 运行完整测试套件
```bash
# 运行所有测试
./test_mcp_gateway/run_all_tests.sh

# 查看帮助
./test_mcp_gateway/run_all_tests.sh --help

# 仅运行设置
./test_mcp_gateway/run_all_tests.sh --setup

# 仅运行清理
./test_mcp_gateway/run_all_tests.sh --cleanup
```

### 运行单独的测试
```bash
# 数据库集成测试
./test_mcp_gateway/test_mysql_integration.sh

# HTTP API 测试
./test_mcp_gateway/test_http_api.sh

# MCP stdio 测试
./test_mcp_gateway/test_mcp_stdio.sh

# MCP HTTP SSE 测试
./test_mcp_gateway/test_mcp_http_sse.sh

# MCP Streamable 测试
./test_mcp_gateway/test_mcp_streamable.sh
```

## 预期结果

成功运行后，您应该看到：

```
🎉 所有测试通过！

您的 mcp_srv_mgr 已准备好与 Unla Gateway 集成！

下一步：
1. 启动 Unla 网关：./mcp-gateway --config test_mcp_gateway/unla-config.yaml
2. 您的服务管理器将通过网关在 http://127.0.0.1:8081 可用
3. 所有协议（HTTP、MCP stdio、MCP HTTP、MCP Streamable）都已配置和测试
```

## 总结

此集成测试套件提供了全面的验证，确保 mcp_srv_mgr 可以成功地通过所有支持的协议与 Unla Gateway 集成。测试确保：

- ✅ **协议兼容性**：所有 MCP 协议变体都能正确工作
- ✅ **工具功能**：服务管理工具按预期运行
- ✅ **数据库集成**：持久存储和缓存正常工作
- ✅ **网关代理**：通过 Unla Gateway 的端到端集成
- ✅ **错误处理**：优雅处理错误条件
- ✅ **性能**：可接受的响应时间和资源使用

运行 `./test_mcp_gateway/run_all_tests.sh` 来执行完整的测试套件，并验证您的集成已准备好用于生产使用。

---

**注意**：本测试套件专为开发和测试环境设计。在生产环境中部署之前，请确保按照安全建议进行适当的配置。