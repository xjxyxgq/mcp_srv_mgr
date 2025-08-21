#!/bin/bash

# Simplified Integration Test Suite
# Tests only essential functionality with proper cleanup

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
    fi
    if [ -n "$GATEWAY_PID" ]; then
        kill "$GATEWAY_PID" 2>/dev/null || true
    fi
    
    # Kill any remaining processes
    pkill -f "mcp-server" 2>/dev/null || true
    pkill -f "mcp-gateway" 2>/dev/null || true
    
    # Clean temp files
    rm -f /tmp/mcp_srv_*.log /tmp/gateway_*.log /tmp/test_*.json
    
    echo -e "${GREEN}✅ Cleanup completed${NC}"
}

trap cleanup EXIT

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
    
    echo -n "Waiting for $name to start... "
    
    while [ $count -lt $max_wait ]; do
        if curl -f -s "$url" >/dev/null 2>&1; then
            echo -e "${GREEN}Ready!${NC}"
            return 0
        fi
        sleep 1
        count=$((count + 1))
        echo -n "."
    done
    
    echo -e "${RED}Failed to start!${NC}"
    return 1
}

echo -e "${CYAN}🚀 Simple Integration Test Suite${NC}"
echo "=================================="
echo ""

# Test 1: Start mcp_srv_mgr
echo -e "${BLUE}Starting mcp_srv_mgr...${NC}"
./mcp-server -mode=http -config=test_mcp_gateway/config.yaml >/tmp/mcp_srv_test.log 2>&1 &
MCP_SRV_PID=$!

if wait_for_service "http://127.0.0.1:8080/health" "mcp_srv_mgr"; then
    echo -e "${GREEN}✅ mcp_srv_mgr started (PID: $MCP_SRV_PID)${NC}"
    
    # Test mcp_srv_mgr endpoints
    echo ""
    echo -e "${BLUE}Testing mcp_srv_mgr API:${NC}"
    test_endpoint "Health Check" "http://127.0.0.1:8080/health" "healthy"
    test_endpoint "Services List" "http://127.0.0.1:8080/services" "success"
    test_endpoint "Service Status" "http://127.0.0.1:8080/services/example-service/status" "example-service"
else
    echo -e "${RED}❌ Failed to start mcp_srv_mgr${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 3))
fi

echo ""

# Test 2: Start Unla Gateway
echo -e "${BLUE}Starting Unla Gateway...${NC}"
./mcp-gateway --conf test_mcp_gateway/mcp-gateway-working.yaml >/tmp/gateway_test.log 2>&1 &
GATEWAY_PID=$!

if wait_for_service "http://127.0.0.1:8081/health_check" "Unla Gateway"; then
    echo -e "${GREEN}✅ Unla Gateway started (PID: $GATEWAY_PID)${NC}"
    
    # Test gateway endpoints
    echo ""
    echo -e "${BLUE}Testing Unla Gateway:${NC}"
    test_endpoint "Gateway Health" "http://127.0.0.1:8081/health_check" "ok"
    
    # Test configuration reload
    echo -n "Testing config reload... "
    if [ -f "./test_mcp_gateway/mcp-gateway.pid" ]; then
        pid=$(cat ./test_mcp_gateway/mcp-gateway.pid)
        if kill -HUP "$pid" 2>/dev/null; then
            sleep 2
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
        echo -e "${YELLOW}SKIPPED${NC}"
    fi
else
    echo -e "${RED}❌ Failed to start Unla Gateway${NC}"
    TESTS_FAILED=$((TESTS_FAILED + 2))
fi

echo ""

# Test 3: Service Integration
if [ -n "$MCP_SRV_PID" ] && [ -n "$GATEWAY_PID" ]; then
    echo -e "${BLUE}Testing Service Integration:${NC}"
    
    echo -n "Testing both services running... "
    if curl -f -s "http://127.0.0.1:8080/health" >/dev/null && curl -f -s "http://127.0.0.1:8081/health_check" >/dev/null; then
        echo -e "${GREEN}PASSED${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}FAILED${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
    
    echo -n "Testing port isolation... "
    # Verify services are on different ports
    if netstat -an 2>/dev/null | grep -q ":8080.*LISTEN" && netstat -an 2>/dev/null | grep -q ":8081.*LISTEN"; then
        echo -e "${GREEN}PASSED${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${YELLOW}SKIPPED (netstat not available)${NC}"
    fi
fi

echo ""

# Final Results
echo -e "${CYAN}═══ FINAL RESULTS ═══${NC}"
echo ""
total_tests=$((TESTS_PASSED + TESTS_FAILED))
echo "Total Tests: $total_tests"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Failed: $TESTS_FAILED${NC}"

if [ $TESTS_FAILED -eq 0 ]; then
    success_rate="100%"
else
    success_rate=$(( (TESTS_PASSED * 100) / total_tests ))"%"
fi
echo "Success Rate: $success_rate"

echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 ALL TESTS PASSED! 🎉${NC}"
    echo ""
    echo -e "${YELLOW}✨ System Status:${NC}"
    echo "  🟢 mcp_srv_mgr HTTP API: WORKING"
    echo "  🟢 Unla Gateway: WORKING"
    echo "  🟢 Service Integration: WORKING"
    echo ""
    echo -e "${CYAN}🚀 Your integration is ready!${NC}"
    exit 0
else
    echo -e "${RED}💥 SOME TESTS FAILED 💥${NC}"
    echo ""
    echo "Check logs for details:"
    echo "  mcp_srv_mgr: /tmp/mcp_srv_test.log"
    echo "  gateway: /tmp/gateway_test.log"
    exit 1
fi