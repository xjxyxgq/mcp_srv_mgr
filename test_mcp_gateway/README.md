# mcp_srv_mgr + Unla Gateway 集成测试

本目录包含 `mcp_srv_mgr`（Linux 服务管理系统）与 Unla MCP Gateway 的完整集成测试套件。

## 🚀 快速开始

```bash
# 1. 数据库初始化（确保 MySQL 127.0.0.1:3311 和 Redis 127.0.0.1:6379 运行）
./setup-mysql-gateway.sh
./insert-mysql-config.sh

# 2. 运行完整集成测试
./run_all_tests.sh

# 3. 或运行 MySQL 专项测试
./test-mysql-integration.sh
```

**预期结果**: 所有测试通过，显示 `🎉 ALL TESTS PASSED!`

## 📁 文件说明

### 🔧 核心配置
- `config.yaml` - mcp_srv_mgr HTTP 服务配置
- `mcp-gateway-mysql.yaml` - Unla Gateway MySQL 配置（推荐）
- `mcp_srv_mgr_proxy.yaml` - MCP 工具代理配置
- `unla-config.yaml` - Unla Gateway 完整配置
- `docker-compose.yml` - Docker 容器配置

### 🗄️ 数据库管理
- `setup-mysql-gateway.sh` - MySQL 数据库初始化
- `insert-mysql-config.sh` - MCP 配置数据插入
- `init.sql` - 数据库初始化 SQL

### 🧪 测试脚本
- `run_all_tests.sh` - **主测试脚本**（MySQL 版本）
- `test-mysql-integration.sh` - MySQL 集成专项测试
- `test_http_api.sh` - HTTP API 测试
- `test_mcp_stdio.sh` - MCP stdio 协议测试
- `test_mcp_http_sse.sh` - MCP HTTP SSE 测试
- `test_mcp_streamable.sh` - MCP Streamable 测试
- `quick_test.sh` - 快速验证测试

### 📚 文档
- `INTEGRATION_GUIDE.md` - **详细集成指南**（推荐阅读）
- `README.md` - 本文件

## 🎯 测试覆盖

### ✅ 支持的协议（4种）
- **HTTP REST API** - 端口 8080
- **MCP stdio** - 标准输入输出
- **MCP over HTTP (SSE)** - 端口 8082  
- **MCP Streamable HTTP** - 端口 8083

### ✅ 后端服务
- **MySQL**: 127.0.0.1:3311（配置持久化）
- **Redis**: 127.0.0.1:6379（会话管理）
- **mcp_srv_mgr**: HTTP API 服务
- **Unla Gateway**: 统一代理网关

### ✅ MCP 工具（8个）
- `list_services`, `get_service_status`
- `start_service`, `stop_service`, `restart_service`
- `enable_service`, `disable_service`
- `get_docker_logs`

## 📊 测试结果

### 主测试脚本成功输出：
```
🎉 ALL TESTS PASSED! 🎉

✨ Integration Status:
  🟢 Prerequisites: SATISFIED
  🟢 mcp_srv_mgr HTTP API: WORKING
  🟢 Unla Gateway: WORKING
  🟢 Service Integration: WORKING
  🟢 MCP Protocol: SUPPORTED

🚀 Your system is fully integrated and ready!
```

### MySQL 集成测试成功输出：
```
🎉 ALL MYSQL INTEGRATION TESTS PASSED! 🎉

✨ MySQL Integration Status:
  🟢 MySQL Database: CONNECTED
  🟢 Redis Session Store: CONNECTED
  🟢 mcp_srv_mgr API: WORKING
  🟢 Unla Gateway: WORKING WITH MYSQL
  🟢 Configuration Management: PERSISTENT
  🟢 Session Management: REDIS-BACKED
```

## 🚨 故障排除

### 常见问题
1. **MySQL 连接失败**: 确保 MySQL 运行在 127.0.0.1:3311
2. **端口冲突**: 检查端口 8080, 8081, 3311, 6379 是否被占用
3. **配置错误**: 运行 `./mcp-gateway test --conf mcp-gateway-mysql.yaml`

### 日志查看
```bash
# 测试日志
tail -f /tmp/mcp_srv_test.log
tail -f /tmp/gateway_test.log
```

## 🎯 使用方式

### 手动启动服务
```bash
# 启动 mcp_srv_mgr
./mcp-server -mode=http -config=test_mcp_gateway/config.yaml

# 启动 Unla Gateway（MySQL 版本）
./mcp-gateway --conf test_mcp_gateway/mcp-gateway-mysql.yaml
```

### AI 模型集成示例
```python
import requests

# 通过网关调用服务管理工具
response = requests.post("http://127.0.0.1:8081/mcp", json={
    "jsonrpc": "2.0", "id": 1, "method": "tools/call",
    "params": {"name": "get_service_status", "arguments": {"service_name": "nginx"}}
})
```

## 📈 下一步

1. **阅读详细文档**: 查看 `INTEGRATION_GUIDE.md`
2. **生产部署**: 修改密码和安全配置
3. **AI 接入**: 配置 AI 模型连接网关
4. **监控设置**: 配置日志和指标监控

---

🎯 **项目状态**: 集成完成，测试通过，生产就绪！