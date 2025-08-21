#!/bin/bash

# 快速测试脚本 - 测试基本功能
set -e

echo "=== 快速协议测试 ==="
echo "测试四种协议的基本功能..."
echo

# 检查当前目录
if [ ! -f "../go.mod" ]; then
    echo "错误: 请从项目根目录的test目录运行此脚本"
    exit 1
fi

# 构建服务器
echo "🔨 构建服务器..."
cd ..
go build -o mcp-server cmd/server/main.go
cd test

echo "✅ 构建完成"
echo

# 测试每个协议的启动
test_protocol() {
    local protocol=$1
    local mode_flag=$2
    local port=$3
    
    echo "🧪 测试 $protocol 协议..."
    
    if [ "$protocol" = "MCP stdio" ]; then
        # 对于stdio模式，只需要测试能否启动
        timeout 3s ../mcp-server -mode=mcp > /dev/null 2>&1 &
        local pid=$!
        sleep 1
        
        if kill -0 $pid 2>/dev/null; then
            echo "✅ $protocol 启动成功"
            kill $pid 2>/dev/null || true
            return 0
        else
            echo "❌ $protocol 启动失败"
            return 1
        fi
    else
        # 为HTTP模式创建配置文件
        cat > "config_test_${port}.yaml" << EOF
server:
  host: "127.0.0.1"
  port: $port
log:
  level: "error"
  format: "text"
  output: "stderr"
EOF

        # 启动服务器
        ../mcp-server $mode_flag -config="config_test_${port}.yaml" > /dev/null 2>&1 &
        local pid=$!
        
        # 等待服务器启动
        sleep 2
        
        # 测试健康检查
        if curl -s -f "http://127.0.0.1:${port}/health" > /dev/null; then
            echo "✅ $protocol 启动并响应成功"
            kill $pid 2>/dev/null || true
            rm -f "config_test_${port}.yaml"
            return 0
        else
            echo "❌ $protocol 启动失败或无响应"
            kill $pid 2>/dev/null || true
            rm -f "config_test_${port}.yaml"
            return 1
        fi
    fi
}

# 测试所有协议
declare -a results=()
declare -a protocols=("MCP stdio" "HTTP REST API" "MCP over HTTP (SSE)" "MCP Streamable HTTP")

# MCP stdio
if test_protocol "MCP stdio" "-mode=mcp" "0"; then
    results+=(0)
else
    results+=(1)
fi

# HTTP REST API  
if test_protocol "HTTP REST API" "-mode=http" "18001"; then
    results+=(0)
else
    results+=(1)
fi

# MCP over HTTP (SSE)
if test_protocol "MCP over HTTP (SSE)" "-mode=mcp-http" "18002"; then
    results+=(0)
else
    results+=(1)
fi

# MCP Streamable HTTP
if test_protocol "MCP Streamable HTTP" "-mode=mcp-streamable" "18003"; then
    results+=(0)
else
    results+=(1)
fi

echo
echo "================================================"
echo "🏁 快速测试结果"
echo "================================================"

passed_count=0
failed_count=0

for i in "${!protocols[@]}"; do
    if [ ${results[$i]} -eq 0 ]; then
        echo "✅ ${protocols[$i]}"
        ((passed_count++))
    else
        echo "❌ ${protocols[$i]}"
        ((failed_count++))
    fi
done

echo
echo "📊 统计: $passed_count/$((passed_count + failed_count)) 个协议正常工作"

# 清理
rm -f config_test_*.yaml
rm -f ../mcp-server

if [ $failed_count -eq 0 ]; then
    echo
    echo "🎉 快速测试通过! 所有四种协议都能正常启动。"
    echo "运行 './run_tests.sh' 进行完整测试。"
    exit 0
else
    echo
    echo "⚠️  有 $failed_count 个协议启动失败。"
    exit 1
fi