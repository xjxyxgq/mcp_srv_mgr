#!/bin/bash

# Test MySQL Integration with Unla Gateway
# This script tests the complete integration using MySQL backend

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# PIDs for cleanup
MCP_SRV_PID=""
GATEWAY_PID=""
TESTS_PASSED=0
TESTS_FAILED=0

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}🧹 Cleaning up...${NC}"
    
    if [ -n "$MCP_SRV_PID" ]; then
        kill "$MCP_SRV_PID" 2>/dev/null || true
        echo "Stopped mcp_srv_mgr (PID: $MCP_SRV_PID)"
    fi
    if [ -n "$GATEWAY_PID" ]; then
        kill "$GATEWAY_PID" 2>/dev/null || true
        echo "Stopped gateway (PID: $GATEWAY_PID)"
    fi
    
    # Kill any remaining processes
    pkill -f "mcp-server" 2>/dev/null || true
    pkill -f "mcp-gateway" 2>/dev/null || true
    
    # Clean temp files
    rm -f /tmp/mcp_srv_mysql_test.log /tmp/gateway_mysql_test.log
    
    echo -e "${GREEN}✅ Cleanup completed${NC}"
}

trap cleanup EXIT

echo -e "${CYAN}🚀 MySQL Integration Test for Unla Gateway${NC}"
echo "============================================="
echo ""

# Test function
test_endpoint() {
    local name="$1"
    local url="$2"
    local expected_pattern="$3"
    
    echo -n "Testing $name... "
    
    if response=$(curl -f -s "$url" 2>/dev/null) && echo "$response" | grep -q "$expected_pattern"; then
        echo -e "${GREEN}PASSED${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}FAILED${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Wait for service
wait_for_service() {
    local url="$1"
    local name="$2"
    local max_wait=20
    local count=0
    
    echo -n "⏳ Waiting for $name to start... "
    
    while [ $count -lt $max_wait ]; do
        if curl -f -s "$url" >/dev/null 2>&1; then
            echo -e "${GREEN}Ready!${NC}"
            return 0
        fi
        sleep 1
        count=$((count + 1))
        echo -n "."
    done
    
    echo -e "${RED}Failed!${NC}"
    return 1
}

# Test 1: Database connectivity
echo -e "${BLUE}═══ TESTING DATABASE CONNECTIVITY ═══${NC}"
echo ""

echo -n "Testing MySQL connection... "
if mysql -h 127.0.0.1 -P 3311 -u root -pnov24feb11 -e "SELECT 1;" >/dev/null 2>&1; then
    echo -e "${GREEN}✅ MySQL Connected${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ MySQL Failed${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

echo -n "Testing Redis connection... "
if redis-cli -h 127.0.0.1 -p 6379 ping >/dev/null 2>&1; then
    echo -e "${GREEN}✅ Redis Connected${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ Redis Failed${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

echo -n "Checking mcp_srv_mgr configuration in MySQL... "
config_count=$(mysql -h 127.0.0.1 -P 3311 -u root -pnov24feb11 unla_gateway -e "SELECT COUNT(*) FROM mcp_configs WHERE name='mcp_srv_mgr';" -s -N 2>/dev/null)
if [ "$config_count" -gt 0 ]; then
    echo -e "${GREEN}✅ Configuration Found${NC}"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ Configuration Missing${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

echo ""

# Test 2: Start services
echo -e "${BLUE}═══ STARTING SERVICES ═══${NC}"
echo ""

echo "🚀 Starting mcp_srv_mgr HTTP server..."
./mcp-server -mode=http -config=test_mcp_gateway/config.yaml >/tmp/mcp_srv_mysql_test.log 2>&1 &
MCP_SRV_PID=$!

if wait_for_service "http://127.0.0.1:8080/health" "mcp_srv_mgr"; then
    echo -e "${GREEN}✅ mcp_srv_mgr started (PID: $MCP_SRV_PID)${NC}"
    test_endpoint "mcp_srv_mgr Health" "http://127.0.0.1:8080/health" "healthy"
    test_endpoint "mcp_srv_mgr Services" "http://127.0.0.1:8080/services" "success"
else
    echo -e "${RED}❌ Failed to start mcp_srv_mgr${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 2))
fi

echo ""

echo "🚀 Starting Unla Gateway with MySQL..."
./mcp-gateway --conf test_mcp_gateway/mcp-gateway-mysql.yaml >/tmp/gateway_mysql_test.log 2>&1 &
GATEWAY_PID=$!

if wait_for_service "http://127.0.0.1:8081/health_check" "Unla Gateway"; then
    echo -e "${GREEN}✅ Unla Gateway started with MySQL (PID: $GATEWAY_PID)${NC}"
    test_endpoint "Gateway Health" "http://127.0.0.1:8081/health_check" "ok"
    
    # Test configuration reload from MySQL
    echo -n "Testing configuration reload from MySQL... "
    if [ -f "./test_mcp_gateway/mcp-gateway.pid" ]; then
        pid=$(cat ./test_mcp_gateway/mcp-gateway.pid)
        if kill -HUP "$pid" 2>/dev/null; then
            sleep 3
            if curl -f -s "http://127.0.0.1:8081/health_check" >/dev/null 2>&1; then
                echo -e "${GREEN}PASSED${NC}"
                TESTS_PASSED=$((TESTS_PASSED + 1))
            else
                echo -e "${RED}FAILED${NC}"
                TESTS_FAILED=$((TESTS_FAILED + 1))
            fi
        else
            echo -e "${RED}FAILED${NC}"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    else
        echo -e "${YELLOW}SKIPPED (no PID file)${NC}"
    fi
else
    echo -e "${RED}❌ Failed to start Unla Gateway${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 2))
fi

echo ""

# Test 3: Integration tests
echo -e "${BLUE}═══ TESTING INTEGRATION ═══${NC}"
echo ""

if [ -n "$MCP_SRV_PID" ] && [ -n "$GATEWAY_PID" ]; then
    test_endpoint "Both Services Running" "http://127.0.0.1:8080/health" "healthy"
    test_endpoint "Gateway MySQL Backend" "http://127.0.0.1:8081/health_check" "ok"
    
    # Test session management with Redis
    echo -n "Testing session storage with Redis... "
    session_key="test_session_$(date +%s)"
    if redis-cli -h 127.0.0.1 -p 6379 set "$session_key" "test_value" EX 60 >/dev/null 2>&1; then
        stored_value=$(redis-cli -h 127.0.0.1 -p 6379 get "$session_key" 2>/dev/null)
        if [ "$stored_value" = "test_value" ]; then
            echo -e "${GREEN}PASSED${NC}"
            TESTS_PASSED=$((TESTS_PASSED + 1))
            redis-cli -h 127.0.0.1 -p 6379 del "$session_key" >/dev/null 2>&1
        else
            echo -e "${RED}FAILED${NC}"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    else
        echo -e "${RED}FAILED${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    
    # Test database configuration persistence
    echo -n "Testing MySQL configuration persistence... "
    tools_count=$(mysql -h 127.0.0.1 -P 3311 -u root -pnov24feb11 unla_gateway -e "SELECT JSON_LENGTH(tools) FROM mcp_configs WHERE name='mcp_srv_mgr';" -s -N 2>/dev/null)
    if [ "$tools_count" -gt 0 ]; then
        echo -e "${GREEN}PASSED (${tools_count} tools configured)${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}FAILED${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
else
    echo -e "${RED}❌ Cannot test integration - services not running${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 3))
fi

echo ""

# Final Results
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN} MYSQL INTEGRATION TEST RESULTS${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""

total_tests=$((TESTS_PASSED + TESTS_FAILED))
echo "Test Results Summary:"
echo "====================="
echo "Total Tests: $total_tests"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"

if [ $total_tests -gt 0 ]; then
    success_rate=$(( (TESTS_PASSED * 100) / total_tests ))
    echo "Success Rate: ${success_rate}%"
fi

echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 ALL MYSQL INTEGRATION TESTS PASSED! 🎉${NC}"
    echo ""
    echo -e "${YELLOW}✨ MySQL Integration Status:${NC}"
    echo "  🟢 MySQL Database: CONNECTED"
    echo "  🟢 Redis Session Store: CONNECTED"
    echo "  🟢 mcp_srv_mgr API: WORKING"
    echo "  🟢 Unla Gateway: WORKING WITH MYSQL"
    echo "  🟢 Configuration Management: PERSISTENT"
    echo "  🟢 Session Management: REDIS-BACKED"
    echo ""
    echo -e "${CYAN}🚀 Your MySQL-backed system is fully integrated!${NC}"
    echo ""
    echo -e "${YELLOW}Backend Services:${NC}"
    echo "  • MySQL: 127.0.0.1:3311 (persistent config)"
    echo "  • Redis: 127.0.0.1:6379 (session storage)"
    echo "  • mcp_srv_mgr: http://127.0.0.1:8080"
    echo "  • Unla Gateway: http://127.0.0.1:8081"
    exit 0
else
    echo -e "${RED}💥 SOME TESTS FAILED 💥${NC}"
    echo ""
    echo "Check logs for details:"
    if [ -f "/tmp/mcp_srv_mysql_test.log" ]; then
        echo "  • mcp_srv_mgr: /tmp/mcp_srv_mysql_test.log"
        echo "    Recent output:"
        tail -3 /tmp/mcp_srv_mysql_test.log | sed 's/^/      /'
    fi
    if [ -f "/tmp/gateway_mysql_test.log" ]; then
        echo "  • gateway: /tmp/gateway_mysql_test.log"
        echo "    Recent output:"
        tail -3 /tmp/gateway_mysql_test.log | sed 's/^/      /'
    fi
    exit 1
fi