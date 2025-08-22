# mcp_srv_mgr + Unla Gateway 集成指南

## 🎯 项目概述

本项目实现了 `mcp_srv_mgr`（Linux 服务管理系统）与 Unla MCP Gateway 的完整集成，通过统一的 MCP 协议为 AI 模型提供系统管理能力。

### 支持的协议
- **HTTP REST API** - 传统的 RESTful 接口
- **MCP stdio** - 标准输入输出 MCP 协议
- **MCP over HTTP (SSE)** - 基于 Server-Sent Events 的 MCP 协议
- **MCP Streamable HTTP** - 支持双向流式传输的 MCP 协议

### 后端架构
- **MySQL**: 127.0.0.1:3311（配置持久化存储）
- **Redis**: 127.0.0.1:6379（会话管理）
- **mcp_srv_mgr**: http://127.0.0.1:8080（服务管理 API）
- **Unla Gateway**: http://127.0.0.1:8081（统一代理网关）

## 🚀 快速开始

### 1. 环境准备
```bash
# 确保 MySQL 和 Redis 服务运行
mysql -h 127.0.0.1 -P 3311 -u root -pnov24feb11 -e "SELECT 1;"
redis-cli -h 127.0.0.1 -p 6379 ping

# 构建 mcp-server（如果未构建）
go build -o mcp-server cmd/server/main.go
```

### 2. 数据库初始化
```bash
# 设置 MySQL 数据库和表结构
./test_mcp_gateway/setup-mysql-gateway.sh

# 插入 MCP 配置数据
./test_mcp_gateway/insert-mysql-config.sh
```

### 3. 启动服务并测试
```bash
# 运行完整集成测试（推荐）
./test_mcp_gateway/run_all_tests.sh

# 或运行专门的 MySQL 集成测试
./test_mcp_gateway/test-mysql-integration.sh
```

### 4. 手动启动服务
```bash
# 启动 mcp_srv_mgr HTTP 服务
./mcp-server -mode=http -config=test_mcp_gateway/config.yaml

# 启动 Unla Gateway（使用 MySQL）
./mcp-gateway --conf test_mcp_gateway/mcp-gateway-mysql.yaml
```

## 📁 文件结构

### 核心配置文件
- `config.yaml` - mcp_srv_mgr HTTP 服务配置
- `mcp-gateway-mysql.yaml` - Unla Gateway MySQL 配置
- `mcp_srv_mgr_proxy.yaml` - MCP 工具代理配置
- `unla-config.yaml` - Unla Gateway 完整配置
- `docker-compose.yml` - Docker 容器配置

### 数据库脚本
- `setup-mysql-gateway.sh` - MySQL 数据库初始化
- `insert-mysql-config.sh` - 配置数据插入
- `init.sql` - 数据库初始化 SQL

### 测试脚本
- `run_all_tests.sh` - 主测试脚本（使用 MySQL）
- `test-mysql-integration.sh` - MySQL 专项集成测试
- `test_http_api.sh` - HTTP API 测试
- `test_mcp_stdio.sh` - MCP stdio 协议测试
- `test_mcp_http_sse.sh` - MCP HTTP SSE 测试
- `test_mcp_streamable.sh` - MCP Streamable 测试
- `quick_test.sh` - 快速验证测试

## 🔧 配置详解

### MySQL 数据库配置
```yaml
# mcp-gateway-mysql.yaml
storage:
  type: "db"
  database:
    type: "mysql"
    host: "127.0.0.1"
    port: 3311
    user: "root"
    password: "nov24feb11"
    dbname: "unla_gateway"
```

### Redis 会话存储配置
```yaml
session:
  type: "redis"
  redis:
    addr: "127.0.0.1:6379"
    password: ""
    db: 0
```

### MCP 工具配置
已配置的 8 个工具：
- `list_services` - 列出系统服务
- `get_service_status` - 获取服务状态
- `start_service` - 启动服务
- `stop_service` - 停止服务
- `restart_service` - 重启服务
- `enable_service` - 启用服务
- `disable_service` - 禁用服务
- `get_docker_logs` - 获取 Docker 日志

## 🧪 测试说明

### 主测试脚本 (`run_all_tests.sh`)
执行完整的集成测试，包括：
- ✅ 环境检查（二进制文件、配置文件）
- ✅ mcp_srv_mgr HTTP 服务测试
- ✅ Unla Gateway 启动和健康检查
- ✅ 服务集成测试
- ✅ MySQL 配置持久化验证
- ✅ MCP 协议基础测试

**预期结果**: 12/12 测试通过，100% 成功率

### MySQL 集成测试 (`test-mysql-integration.sh`)
专门测试 MySQL 后端集成：
- ✅ MySQL/Redis 连接测试
- ✅ 配置数据验证
- ✅ 服务启动和代理测试
- ✅ 会话存储测试
- ✅ 配置持久化测试

**预期结果**: 11/11 测试通过，100% 成功率

## 🎯 使用场景

### 1. AI 模型集成
```python
# Python 示例：通过 Unla Gateway 调用服务管理工具
import requests

gateway_url = "http://127.0.0.1:8081/mcp"

# 获取服务状态
response = requests.post(gateway_url, json={
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
        "name": "get_service_status",
        "arguments": {"service_name": "nginx"}
    }
})
```

### 2. 直接 API 访问
```bash
# 直接访问 mcp_srv_mgr API
curl http://127.0.0.1:8080/services
curl http://127.0.0.1:8080/services/nginx/status

# 通过网关访问（MCP 协议）
curl -X POST http://127.0.0.1:8081/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{}}}'
```

### 3. 配置热重载
```bash
# 修改 MySQL 中的配置后重载
kill -HUP $(cat test_mcp_gateway/mcp-gateway.pid)
```

## 🚨 故障排除

### 常见问题

**1. MySQL 连接失败**
```bash
# 检查 MySQL 服务
mysql -h 127.0.0.1 -P 3311 -u root -pnov24feb11 -e "SELECT 1;"
```

**2. 端口冲突**
```bash
# 检查端口占用
lsof -i :8080 :8081 :3311 :6379
```

**3. 网关启动失败**
```bash
# 测试配置文件
./mcp-gateway test --conf test_mcp_gateway/mcp-gateway-mysql.yaml
```

**4. 权限问题**
```bash
# 确保脚本可执行
chmod +x test_mcp_gateway/*.sh
chmod +x mcp-server mcp-gateway
```

### 日志查看
```bash
# 服务日志
tail -f /tmp/mcp_srv_test.log
tail -f /tmp/gateway_test.log

# MySQL 日志（如果使用 Docker）
docker compose -f test_mcp_gateway/docker-compose.yml logs mysql
```

## 🎉 成功验证

当您看到以下输出时，说明集成完全成功：

```
🎉 ALL TESTS PASSED! 🎉

✨ Integration Status:
  🟢 Prerequisites: SATISFIED
  🟢 mcp_srv_mgr HTTP API: WORKING
  🟢 Unla Gateway: WORKING
  🟢 Service Integration: WORKING
  🟢 MCP Protocol: SUPPORTED

🚀 Your system is fully integrated and ready!

Available endpoints:
  • mcp_srv_mgr: http://127.0.0.1:8080
  • Unla Gateway: http://127.0.0.1:8081
```

## 📈 下一步

1. **生产部署**：修改配置中的密码和安全设置
2. **AI 模型接入**：配置 Claude Code 或其他 AI 工具连接网关
3. **监控设置**：配置日志、指标监控
4. **扩展功能**：添加更多系统管理工具

---

🎯 **总结**: 本集成方案提供了完整的 Linux 服务管理能力，通过统一的 MCP 协议为 AI 模型提供强大的系统管理工具，支持持久化配置、会话管理和多协议访问。