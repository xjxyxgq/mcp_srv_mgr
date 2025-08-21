# MCP服务管理器 - 四协议测试套件

这个测试套件全面测试MCP服务管理器的四种协议实现：

1. **MCP stdio** - 原生MCP协议（stdin/stdout）
2. **HTTP REST API** - 传统RESTful API
3. **MCP over HTTP (SSE)** - 使用Server-Sent Events的MCP协议
4. **MCP Streamable HTTP** - 支持双向流式传输的MCP协议

## 文件结构

```
test/
├── README.md              # 本文件
├── helper.go              # 测试助手和工具函数
├── mcp_stdio_test.go      # MCP stdio协议测试
├── http_rest_test.go      # HTTP REST API协议测试
├── mcp_sse_test.go        # MCP over HTTP (SSE)协议测试
├── mcp_streamable_test.go # MCP Streamable HTTP协议测试
├── integration_test.go    # 集成测试和协议兼容性测试
├── run_tests.sh          # 完整测试脚本
└── quick_test.sh         # 快速测试脚本
```

## 运行测试

### 前置条件

1. 确保已安装Go 1.21或更高版本
2. 确保在项目根目录运行测试

### 快速测试

快速验证所有四种协议是否能正常启动：

```bash
cd test
./quick_test.sh
```

### 完整测试

运行所有测试，包括功能测试、集成测试和兼容性测试：

```bash
cd test
./run_tests.sh
```

### 运行特定测试

```bash
# 运行单个协议测试
go test -v -run TestMCPStdio
go test -v -run TestHTTPREST
go test -v -run TestMCPSSE
go test -v -run TestMCPStreamable

# 运行集成测试
go test -v -run TestAllProtocolsIntegration

# 运行兼容性测试
go test -v -run TestProtocolCompatibility
```

### 运行负载测试

```bash
# 设置环境变量启用负载测试
RUN_LOAD_TESTS=true ./run_tests.sh

# 或者直接运行包含负载测试的测试
go test -v -run "WithLoad|Concurrent"
```

### 短测试模式

跳过长时间运行的测试：

```bash
go test -v -short
```

## 测试覆盖

### MCP stdio协议测试
- 服务器启动和连接
- 初始化协议握手
- 工具列表获取
- 工具调用（list_services）
- 提示词列表
- 错误处理
- 多轮交互稳定性

### HTTP REST API协议测试
- 服务器启动和健康检查
- 服务列表API（/services）
- 服务过滤（按类型）
- 服务状态查询
- 服务操作API
- CORS支持
- 错误处理
- 并发请求负载测试

### MCP over HTTP (SSE)协议测试
- SSE连接建立
- 会话管理
- MCP初始化
- 工具列表和调用
- 心跳机制
- 多客户端并发
- 连接超时和重连

### MCP Streamable HTTP协议测试
- 健康检查
- 单次请求-响应模式
- 双向流式连接
- 流式工具调用
- 多请求流水线
- 会话管理
- 并发客户端测试

### 集成测试
- 所有协议同时启动
- 并发协议测试
- 资源使用监控
- 优雅关闭测试

### 兼容性测试
- MCP协议间的工具列表一致性
- 响应格式兼容性
- 功能对等性验证

## 测试环境

测试使用不同的端口避免冲突：

- HTTP REST API: 8081-8082
- MCP over HTTP (SSE): 8083-8084  
- MCP Streamable HTTP: 8085-8086
- 集成测试: 9001-9005
- 兼容性测试: 9004-9005
- 快速测试: 18001-18003

## 故障排除

### 常见问题

1. **端口占用错误**
   ```
   listen tcp :8081: bind: address already in use
   ```
   解决：等待几秒后重试，或使用 `lsof -ti:8081 | xargs kill` 清理端口

2. **服务器启动超时**
   ```
   server failed to start: timeout
   ```
   解决：检查系统资源，确保没有防火墙阻止连接

3. **MCP stdio测试失败**
   ```
   Failed to read initialize response
   ```
   解决：检查服务器二进制文件是否正确构建

4. **权限错误**
   ```
   permission denied
   ```
   解决：确保脚本有执行权限 `chmod +x *.sh`

### 调试模式

设置环境变量启用详细日志：

```bash
export DEBUG=1
./run_tests.sh
```

### 手动测试

可以手动启动服务器进行测试：

```bash
# 构建服务器
go build -o mcp-server cmd/server/main.go

# 启动不同模式的服务器
./mcp-server -mode=mcp           # MCP stdio
./mcp-server -mode=http          # HTTP REST API  
./mcp-server -mode=mcp-http      # MCP over HTTP (SSE)
./mcp-server -mode=mcp-streamable # MCP Streamable HTTP
```

## 贡献

添加新测试时请遵循以下规范：

1. 为每个新功能添加对应的测试
2. 使用描述性的测试名称
3. 添加适当的超时处理
4. 清理测试资源
5. 更新本README文档

## 性能基准

在标准开发机器上的预期测试时间：

- 快速测试: < 10秒
- 单个协议测试: 30-60秒
- 完整测试套件: 3-5分钟
- 包含负载测试: 5-10分钟