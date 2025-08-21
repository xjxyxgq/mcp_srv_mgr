#!/bin/bash

# Simple test to verify basic functionality
echo "🧪 Simple Integration Test"
echo "=========================="

cd /Users/xuguoqiang/SynologyDrive/Backup/MI_office_notebook/D/myworkspace/nucc_workspace/program/src/nucc.com/mcp_srv_mgr

# Test 1: Build check
echo ""
echo "1. Build check..."
if [ -f "./mcp-server" ]; then
    echo "✅ mcp-server binary exists"
else
    echo "❌ mcp-server binary not found"
    exit 1
fi

# Test 2: MySQL check
echo ""
echo "2. Database check..."
if mysql -h 127.0.0.1 -P 3311 -u root -pnov24feb11 -e "SELECT 1;" > /dev/null 2>&1; then
    echo "✅ MySQL connection works"
else
    echo "❌ MySQL connection failed"
fi

# Test 3: HTTP server test
echo ""
echo "3. HTTP server test..."
echo "Starting server in background..."
./mcp-server -mode=http -config=test_mcp_gateway/config.yaml > /tmp/server.log 2>&1 &
SERVER_PID=$!

sleep 2
echo "Testing health endpoint..."

if curl -f http://127.0.0.1:8080/health > /dev/null 2>&1; then
    echo "✅ HTTP server works"
else
    echo "❌ HTTP server failed"
    echo "Server log:"
    cat /tmp/server.log | head -10
fi

# Cleanup
echo "Stopping server..."
kill $SERVER_PID 2>/dev/null || true

# Test 4: MCP stdio test
echo ""
echo "4. MCP stdio test..."
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}}}}' | timeout 5 ./mcp-server -mode=mcp > /tmp/mcp.log 2>&1

if grep -q '"result"' /tmp/mcp.log; then
    echo "✅ MCP stdio works"
else
    echo "❌ MCP stdio failed"
    echo "MCP response:"
    cat /tmp/mcp.log | head -5
fi

echo ""
echo "🏁 Simple test completed!"
echo ""
echo "If all tests passed, your mcp_srv_mgr is ready for Unla Gateway integration!"