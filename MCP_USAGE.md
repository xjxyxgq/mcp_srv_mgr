# Linux服务管理器 MCP 使用指南

## 什么是MCP?

MCP（Model Context Protocol，模型上下文协议）是一个用于AI模型与外部工具集成的开放协议。通过MCP，Claude等大语言模型可以安全地调用外部工具和服务，实现更强大的功能。

本项目实现了一个专门用于Linux服务管理的MCP服务器，让AI模型能够通过自然语言对话来管理系统服务。

## 主要特性

### 🔧 工具(Tools) 

本MCP服务器提供以下工具供大模型使用：

#### 服务管理工具
- **`list_services`** - 列出所有可用服务
- **`get_service_status`** - 获取特定服务的详细状态
- **`start_service`** - 启动服务
- **`stop_service`** - 停止服务
- **`restart_service`** - 重启服务
- **`enable_service`** - 启用服务(开机自启)
- **`disable_service`** - 禁用服务

#### Docker专用工具
- **`get_docker_logs`** - 获取Docker容器日志

### 🤖 提示词(Prompts)

预定义的提示词模板，帮助大模型更好地理解服务管理场景：

- **`service_management_help`** - 服务管理帮助文档
- **`service_troubleshooting`** - 服务故障排除指导

## 配置方法

### 1. 编译MCP服务器

```bash
cd mcp_srv_mgr
go build -o mcp-server
```

### 2. 配置Claude Desktop

在Claude Desktop配置文件中添加MCP服务器：

**macOS路径**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows路径**: `%APPDATA%/Claude/claude_desktop_config.json`
**Linux路径**: `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "linux-service-manager": {
      "command": "/full/path/to/your/mcp-server",
      "args": ["-mcp", "-log-level", "error"],
      "env": {}
    }
  }
}
```

### 3. 重启Claude Desktop

配置完成后重启Claude Desktop，MCP工具将自动可用。

## 使用示例

### 基础服务管理

**场景1：查看服务状态**
```
👤 用户: "显示所有systemd服务的状态"
🤖 AI: [调用 list_services 工具，参数 service_type: "systemd"]
返回格式化的服务列表，显示每个服务的名称、状态和描述
```

**场景2：检查特定服务**
```
👤 用户: "nginx服务现在是什么状态？"
🤖 AI: [调用 get_service_status 工具，参数 service_name: "nginx"]
返回nginx的详细状态信息，包括PID、运行时间等
```

**场景3：管理Docker容器**
```
👤 用户: "启动docker容器web-app"
🤖 AI: [调用 start_service 工具，参数 service_name: "web-app", service_type: "docker"]
启动容器并返回最新状态
```

### 故障排除场景

**场景1：服务启动问题**
```
👤 用户: "我的mysql服务启动失败了，请帮我诊断"
🤖 AI: [调用 service_troubleshooting 提示词获取指导，然后使用 get_service_status 工具检查当前状态]
提供系统性的故障排除步骤和具体的状态信息
```

**场景2：查看日志**
```
👤 用户: "查看nginx容器的最近100行日志"
🤖 AI: [调用 get_docker_logs 工具，参数 container_name: "nginx", lines: 100]
返回格式化的日志内容，帮助分析问题
```

### 批量操作场景

**场景1：批量管理**
```
👤 用户: "重启所有web相关的服务"
🤖 AI: [先调用 list_services 查找包含"web"关键词的服务，然后依次调用 restart_service]
智能识别相关服务并执行批量操作
```

**场景2：系统维护**
```
👤 用户: "停止所有非必要的服务来释放内存"
🤖 AI: [调用 list_services 获取服务列表，分析哪些是非必要服务，然后询问确认后执行停止操作]
安全地进行系统维护操作
```

## 工具参数说明

### list_services
```json
{
  "service_type": "systemd|sysv|docker"  // 可选，筛选服务类型
}
```

### get_service_status / start_service / stop_service / restart_service / enable_service / disable_service
```json
{
  "service_name": "服务名称",           // 必需
  "service_type": "systemd|sysv|docker"  // 可选，指定服务类型
}
```

### get_docker_logs
```json
{
  "container_name": "容器名称",  // 必需
  "lines": 100                  // 可选，日志行数，默认100
}
```

## 提示词参数说明

### service_management_help
```json
{
  "topic": "systemd|sysv|docker|troubleshooting"  // 可选，具体主题
}
```

### service_troubleshooting
```json
{
  "service_name": "服务名称",        // 必需
  "error_description": "错误描述"    // 可选
}
```

## 安全注意事项

1. **权限要求**: MCP服务器需要适当的权限来管理服务
2. **访问控制**: 确保只有授权用户可以访问MCP服务器
3. **日志监控**: 建议启用日志记录来监控操作
4. **备份**: 在进行重要服务操作前做好备份

## 常见问题排除

### ❌ MCP服务器无法启动

**问题症状**: 执行`./mcp-server -mcp`后没有响应或出错

**解决方案**:
```bash
# 1. 检查Go环境
go version

# 2. 重新编译
go build -o mcp-server

# 3. 检查文件权限
chmod +x mcp-server

# 4. 使用调试模式查看详细错误
./mcp-server -mcp -log-level debug
```

### ❌ Claude Desktop无法连接MCP服务器

**问题症状**: Claude Desktop中看不到MCP工具

**解决方案**:
```json
// 1. 检查配置文件格式 (claude_desktop_config.json)
{
  "mcpServers": {
    "linux-service-manager": {
      "command": "/完整的绝对路径/mcp-server",
      "args": ["-mcp", "-log-level", "error"]
    }
  }
}

// 2. 确认路径是绝对路径，不是相对路径
// 3. 重启Claude Desktop
// 4. 查看Claude Desktop的错误日志
```

### ❌ 服务操作失败

**问题症状**: AI调用工具时返回权限错误

**解决方案**:
```bash
# 1. 检查当前用户权限
whoami
groups

# 2. 确认服务管理器可用
systemctl --version  # 检查systemd
docker --version     # 检查docker

# 3. 必要时使用sudo权限运行
sudo ./mcp-server -mcp

# 4. 检查系统日志
journalctl -u <service-name> -f
```

### ❌ Docker相关操作失败

**问题症状**: Docker容器操作无响应

**解决方案**:
```bash
# 1. 检查Docker服务状态
systemctl status docker

# 2. 确认用户在docker组中
sudo usermod -aG docker $USER
# 注销重新登录生效

# 3. 测试Docker连接
docker ps
```

## 开发和扩展

### 添加新工具
1. 在 `mcp_server.go` 的 `handleListTools` 中添加工具定义
2. 在 `handleCallTool` 中添加工具处理逻辑
3. 实现具体的工具函数

### 添加新提示词
1. 在 `handleListPrompts` 中添加提示词定义
2. 在 `handleGetPrompt` 中添加提示词处理逻辑
3. 实现提示词内容生成函数

这个MCP服务器为AI模型提供了强大的Linux服务管理能力，让用户可以通过自然语言与AI对话来管理系统服务。