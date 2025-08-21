#!/usr/bin/env bash

# Quick test script for testing without Docker containers
# This script assumes MySQL is already running on localhost:3311

set -e

echo "🧪 Quick Test for mcp_srv_mgr Integration"
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
MCP_SRV_MGR_DIR="/Users/xuguoqiang/SynologyDrive/Backup/MI_office_notebook/D/myworkspace/nucc_workspace/program/src/nucc.com/mcp_srv_mgr"
MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3311"
MYSQL_USER="root"
MYSQL_PASS="nov24feb11"

# Test counters
PASSED=0
FAILED=0

print_test_status() {
    local test_name="$1"
    local status="$2"
    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✅ $test_name: PASSED${NC}"
        ((PASSED++))
    else
        echo -e "${RED}❌ $test_name: FAILED${NC}"
        ((FAILED++))
    fi
}

cd "$MCP_SRV_MGR_DIR"

echo ""
echo "🔍 Checking Basic Prerequisites"
echo "-------------------------------"

# Check if mcp-server exists
if [ -f "./mcp-server" ]; then
    print_test_status "mcp-server binary exists" "PASS"
else
    echo "Building mcp-server..."
    if go build -o mcp-server cmd/server/main.go; then
        print_test_status "mcp-server build" "PASS"
    else
        print_test_status "mcp-server build" "FAIL"
        exit 1
    fi
fi

# Test MySQL connection (if available)
echo ""
echo "🗄️ Testing Database Connection"
echo "-------------------------------"

if mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" -e "SELECT 1;" > /dev/null 2>&1; then
    print_test_status "MySQL Connection" "PASS"
    
    # Test database creation
    if mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" -e "CREATE DATABASE IF NOT EXISTS unla_gateway;" > /dev/null 2>&1; then
        print_test_status "Database Creation" "PASS"
    else
        print_test_status "Database Creation" "FAIL"
    fi
else
    print_test_status "MySQL Connection" "FAIL"
    echo "  ⚠️  MySQL not available. Database tests will be skipped."
fi

echo ""
echo "🚀 Testing mcp-server Basic Functionality"
echo "------------------------------------------"

# Test HTTP mode
echo "Testing HTTP mode..."
./mcp-server -mode=http -config=test_mcp_gateway/config.yaml > /tmp/http_test.log 2>&1 &
HTTP_PID=$!

sleep 3

if curl -f http://127.0.0.1:8080/health > /dev/null 2>&1; then
    print_test_status "HTTP Server Start" "PASS"
else
    print_test_status "HTTP Server Start" "FAIL"
    echo "  Log: $(cat /tmp/http_test.log)"
fi

# Clean up HTTP server
kill $HTTP_PID 2>/dev/null || true
wait $HTTP_PID 2>/dev/null || true

# Test MCP stdio mode
echo "Testing MCP stdio mode..."
init_message='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}}}}'

if echo "$init_message" | timeout 10 ./mcp-server -mode=mcp > /tmp/mcp_test.log 2>&1; then
    if grep -q '"result"' /tmp/mcp_test.log; then
        print_test_status "MCP stdio Mode" "PASS"
    else
        print_test_status "MCP stdio Mode" "FAIL"
        echo "  Response: $(cat /tmp/mcp_test.log)"
    fi
else
    print_test_status "MCP stdio Mode" "FAIL"
    echo "  Failed to initialize MCP session"
fi

echo ""
echo "📊 Quick Test Results"
echo "====================="
echo "Passed: $PASSED"
echo "Failed: $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 Basic functionality works!${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Make sure MySQL is running on localhost:3311 (root/nov24feb11)"
    echo "2. Start Docker Desktop to enable container-based tests"
    echo "3. Run the full test suite: ./test_mcp_gateway/run_all_tests.sh"
    echo ""
    echo "Or run individual tests:"
    echo "  ./test_mcp_gateway/test_mysql_integration.sh"
    echo "  ./test_mcp_gateway/test_http_api.sh"
    echo ""
    exit 0
else
    echo -e "${RED}💥 Some basic tests failed!${NC}"
    echo "Please check the issues above before proceeding."
    exit 1
fi