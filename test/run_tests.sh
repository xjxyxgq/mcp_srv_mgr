#!/bin/bash

# 四协议测试脚本
# 测试MCP服务管理器的所有四种协议

set -e

echo "=== MCP 服务管理器四协议测试 ==="
echo "测试以下协议："
echo "1. MCP stdio (原生协议)"
echo "2. HTTP REST API"
echo "3. MCP over HTTP (SSE)"
echo "4. MCP Streamable HTTP"
echo

# 检查当前目录
if [ ! -f "../go.mod" ]; then
    echo "错误: 请从项目根目录的test目录运行此脚本"
    exit 1
fi

# 构建服务器二进制文件
echo "🔨 构建服务器..."
cd ..
go build -o mcp-server cmd/server/main.go
if [ $? -ne 0 ]; then
    echo "❌ 构建失败"
    exit 1
fi
echo "✅ 构建成功"

# 回到测试目录
cd test

# 清理可能存在的配置文件
rm -f config_*.yaml

echo
echo "🧪 运行测试..."
echo "================================================"

# 设置测试环境变量
export CGO_ENABLED=1

# 运行测试的函数
run_test() {
    local test_name=$1
    local test_pattern=$2
    
    echo
    echo "📋 运行 $test_name..."
    echo "----------------------------------------"
    
    if go test -v -run "$test_pattern" -timeout 5m .; then
        echo "✅ $test_name 通过"
        return 0
    else
        echo "❌ $test_name 失败"
        return 1
    fi
}

# 记录测试结果
declare -a results=()
declare -a test_names=()

# 单独运行每个协议的测试
test_names+=("MCP stdio 协议测试")
if run_test "MCP stdio 协议测试" "TestMCPStdio"; then
    results+=(0)
else
    results+=(1)
fi

test_names+=("HTTP REST API 协议测试")
if run_test "HTTP REST API 协议测试" "TestHTTPREST"; then
    results+=(0)
else
    results+=(1)
fi

test_names+=("MCP over HTTP (SSE) 协议测试")
if run_test "MCP over HTTP (SSE) 协议测试" "TestMCPSSE"; then
    results+=(0)
else
    results+=(1)
fi

test_names+=("MCP Streamable HTTP 协议测试")
if run_test "MCP Streamable HTTP 协议测试" "TestMCPStreamable"; then
    results+=(0)
else
    results+=(1)
fi

# 运行集成测试
test_names+=("集成测试 (所有协议)")
if run_test "集成测试 (所有协议)" "TestAllProtocolsIntegration"; then
    results+=(0)
else
    results+=(1)
fi

test_names+=("协议兼容性测试")
if run_test "协议兼容性测试" "TestProtocolCompatibility"; then
    results+=(0)
else
    results+=(1)
fi

# 运行负载测试 (可选)
if [ "${RUN_LOAD_TESTS:-false}" = "true" ]; then
    test_names+=("负载测试")
    if run_test "负载测试" "WithLoad|Concurrent"; then
        results+=(0)
    else
        results+=(1)
    fi
fi

echo
echo "================================================"
echo "🏁 测试结果汇总"
echo "================================================"

passed_count=0
failed_count=0

for i in "${!test_names[@]}"; do
    if [ ${results[$i]} -eq 0 ]; then
        echo "✅ ${test_names[$i]}"
        ((passed_count++))
    else
        echo "❌ ${test_names[$i]}"
        ((failed_count++))
    fi
done

echo
echo "📊 统计:"
echo "  通过: $passed_count"
echo "  失败: $failed_count"
echo "  总计: $((passed_count + failed_count))"

# 清理
echo
echo "🧹 清理测试文件..."
rm -f config_*.yaml
rm -f ../mcp-server

# 退出状态
if [ $failed_count -eq 0 ]; then
    echo
    echo "🎉 所有测试通过! 四种协议都正常工作。"
    exit 0
else
    echo
    echo "⚠️  有 $failed_count 个测试失败。请检查上面的错误信息。"
    exit 1
fi