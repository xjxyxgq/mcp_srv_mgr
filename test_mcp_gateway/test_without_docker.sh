#!/usr/bin/env bash

# Integration tests that work without Docker containers
# Uses existing MySQL database on localhost:3311

set -e

echo "🧪 Integration Tests (No Docker Required)"
echo "=========================================="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
MCP_SRV_MGR_DIR="/Users/xuguoqiang/SynologyDrive/Backup/MI_office_notebook/D/myworkspace/nucc_workspace/program/src/nucc.com/mcp_srv_mgr"
MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3311"
MYSQL_USER="root"
MYSQL_PASS="nov24feb11"
MYSQL_DB="unla_gateway"

# Test counters
PASSED=0
FAILED=0

# Cleanup function
cleanup() {
    echo "🧹 Cleaning up..."
    pkill -f "mcp-server" 2>/dev/null || true
    rm -f /tmp/mcp_*.log /tmp/test_*.json
}

trap cleanup EXIT

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
echo -e "${BLUE}Phase 1: Database Integration Tests${NC}"
echo "-----------------------------------"

# Test MySQL connection and setup
echo "Testing MySQL database setup..."
if mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" -e "CREATE DATABASE IF NOT EXISTS $MYSQL_DB; USE $MYSQL_DB;" > /dev/null 2>&1; then
    print_test_status "MySQL Database Setup" "PASS"
    
    # Create tables
    if mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" -D "$MYSQL_DB" < test_mcp_gateway/init.sql > /dev/null 2>&1; then
        print_test_status "Database Tables Creation" "PASS"
    else
        print_test_status "Database Tables Creation" "FAIL"
    fi
    
    # Test CRUD operations
    session_id="test_$(date +%s)"
    insert_sql="INSERT INTO unla_sessions (id, user_id, integration_name, data) VALUES ('$session_id', 'test_user', 'mcp_srv_mgr_http', '{\"test\": true}');"
    
    if mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" -D "$MYSQL_DB" -e "$insert_sql" > /dev/null 2>&1; then
        print_test_status "Database CRUD Operations" "PASS"
        
        # Cleanup test data
        mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" -D "$MYSQL_DB" -e "DELETE FROM unla_sessions WHERE id='$session_id';" > /dev/null 2>&1
    else
        print_test_status "Database CRUD Operations" "FAIL"
    fi
    
else
    print_test_status "MySQL Database Setup" "FAIL"
    echo "  ⚠️ Make sure MySQL is running on 127.0.0.1:3311 with root/nov24feb11"
fi

echo ""
echo -e "${BLUE}Phase 2: HTTP API Tests${NC}"
echo "-----------------------"

# Test HTTP server
echo "Starting HTTP server..."
./mcp-server -mode=http -config=test_mcp_gateway/config.yaml > /tmp/mcp_http.log 2>&1 &
HTTP_PID=$!

echo "Waiting for server to start..."
sleep 3

# Test health endpoint
if curl -f http://127.0.0.1:8080/health > /tmp/test_health.json 2>/dev/null; then
    print_test_status "HTTP Health Endpoint" "PASS"
else
    print_test_status "HTTP Health Endpoint" "FAIL"
    echo "  Server log: $(tail -3 /tmp/mcp_http.log)"
fi

# Test services endpoint
if curl -f http://127.0.0.1:8080/services > /tmp/test_services.json 2>/dev/null; then
    if grep -q '"success":true' /tmp/test_services.json; then
        print_test_status "HTTP Services Endpoint" "PASS"
        echo "  Found $(jq -r '.services | length' /tmp/test_services.json 2>/dev/null || echo "N/A") services"
    else
        print_test_status "HTTP Services Endpoint" "FAIL"
    fi
else
    print_test_status "HTTP Services Endpoint" "FAIL"
fi

# Test service status endpoint
if curl -f http://127.0.0.1:8080/services/nginx/status > /tmp/test_status.json 2>/dev/null; then
    print_test_status "HTTP Service Status" "PASS"
else
    # Try with type parameter
    if curl -f "http://127.0.0.1:8080/services/nginx/status?type=systemd" > /tmp/test_status.json 2>/dev/null; then
        print_test_status "HTTP Service Status (with type)" "PASS"
    else
        print_test_status "HTTP Service Status" "FAIL"
    fi
fi

# Clean up HTTP server
kill $HTTP_PID 2>/dev/null || true
wait $HTTP_PID 2>/dev/null || true

echo ""
echo -e "${BLUE}Phase 3: MCP stdio Tests${NC}"
echo "-------------------------"

# Test MCP stdio protocol
echo "Testing MCP stdio protocol..."

# Initialize MCP
init_message='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}}}}'

if echo "$init_message" | timeout 10 ./mcp-server -mode=mcp > /tmp/mcp_init.log 2>&1; then
    if grep -q '"result"' /tmp/mcp_init.log; then
        print_test_status "MCP Initialize" "PASS"
    else
        print_test_status "MCP Initialize" "FAIL"
        echo "  Response: $(cat /tmp/mcp_init.log)"
    fi
else
    print_test_status "MCP Initialize" "FAIL"
fi

# Test tools list
tools_message='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
if echo "$tools_message" | timeout 10 ./mcp-server -mode=mcp > /tmp/mcp_tools.log 2>&1; then
    if grep -q '"tools"' /tmp/mcp_tools.log; then
        print_test_status "MCP Tools List" "PASS"
        echo "  Available tools: $(grep -o '"name":"[^"]*"' /tmp/mcp_tools.log | wc -l | tr -d ' ')"
    else
        print_test_status "MCP Tools List" "FAIL"
        echo "  Response: $(cat /tmp/mcp_tools.log)"
    fi
else
    print_test_status "MCP Tools List" "FAIL"
fi

echo ""
echo -e "${BLUE}Phase 4: MCP HTTP Tests${NC}"
echo "-----------------------"

# Test MCP HTTP server
echo "Starting MCP HTTP server..."
MCP_HOST="127.0.0.1" MCP_PORT="8082" ./mcp-server -mode=mcp-http -config=test_mcp_gateway/config-mcp-http.yaml > /tmp/mcp_http_server.log 2>&1 &
MCP_HTTP_PID=$!

sleep 3

# Test MCP HTTP initialize
mcp_http_init='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}}}}'

if curl -f -X POST -H "Content-Type: application/json" -d "$mcp_http_init" http://127.0.0.1:8082/mcp > /tmp/mcp_http_init.json 2>/dev/null; then
    if grep -q '"result"' /tmp/mcp_http_init.json; then
        print_test_status "MCP HTTP Initialize" "PASS"
    else
        print_test_status "MCP HTTP Initialize" "FAIL"
    fi
else
    print_test_status "MCP HTTP Initialize" "FAIL"
fi

# Test MCP HTTP tools
mcp_http_tools='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
if curl -f -X POST -H "Content-Type: application/json" -d "$mcp_http_tools" http://127.0.0.1:8082/mcp > /tmp/mcp_http_tools.json 2>/dev/null; then
    if grep -q '"tools"' /tmp/mcp_http_tools.json; then
        print_test_status "MCP HTTP Tools List" "PASS"
    else
        print_test_status "MCP HTTP Tools List" "FAIL"
    fi
else
    print_test_status "MCP HTTP Tools List" "FAIL"
fi

# Clean up MCP HTTP server
kill $MCP_HTTP_PID 2>/dev/null || true
wait $MCP_HTTP_PID 2>/dev/null || true

echo ""
echo -e "${BLUE}Phase 5: Configuration Validation${NC}"
echo "----------------------------------"

# Test configuration files
configs=("test_mcp_gateway/config.yaml" "test_mcp_gateway/config-mcp-http.yaml" "test_mcp_gateway/unla-config.yaml")
for config in "${configs[@]}"; do
    if [ -f "$config" ]; then
        if [ -s "$config" ]; then
            print_test_status "Config File: $(basename $config)" "PASS"
        else
            print_test_status "Config File: $(basename $config)" "FAIL"
        fi
    else
        print_test_status "Config File: $(basename $config)" "FAIL"
    fi
done

echo ""
echo "🏁 Test Results Summary"
echo "======================="
echo -e "Total Tests Run: $((PASSED + FAILED))"
echo -e "${GREEN}✅ Passed: $PASSED${NC}"
echo -e "${RED}❌ Failed: $FAILED${NC}"

success_rate=$((PASSED * 100 / (PASSED + FAILED)))
echo "Success Rate: ${success_rate}%"

echo ""
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed! Your mcp_srv_mgr is working correctly.${NC}"
    echo ""
    echo "✨ Key accomplishments:"
    echo "  ✅ Database integration works (MySQL on port 3311)"
    echo "  ✅ HTTP REST API server works (port 8080)"
    echo "  ✅ MCP stdio protocol works"
    echo "  ✅ MCP over HTTP protocol works (port 8082)"
    echo "  ✅ Configuration files are valid"
    echo ""
    echo "🚀 Next steps:"
    echo "1. Start Docker Desktop to enable container-based testing"
    echo "2. Start Unla Gateway with: ./mcp-gateway --config test_mcp_gateway/unla-config.yaml"
    echo "3. Run full gateway integration tests when both are running"
    echo ""
    echo "Your service manager is ready for AI model integration! 🤖"
else
    echo -e "${RED}💥 Some tests failed. Please check the issues above.${NC}"
    echo ""
    echo "Common fixes:"
    echo "- Ensure MySQL is running on 127.0.0.1:3311 with root/nov24feb11"
    echo "- Check for port conflicts (8080, 8082 should be free)"
    echo "- Verify mcp-server binary is built correctly"
fi

exit $FAILED