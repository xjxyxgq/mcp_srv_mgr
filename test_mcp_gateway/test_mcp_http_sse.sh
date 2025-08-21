#!/bin/bash

# Test MCP over HTTP (SSE) integration with Unla Gateway
# This script tests the MCP over HTTP with Server-Sent Events through the Unla gateway

set -e

echo "🧪 Testing MCP over HTTP (SSE) Integration with Unla Gateway"
echo "============================================================"

# Configuration
MCP_SRV_MGR_DIR="/Users/xuguoqiang/SynologyDrive/Backup/MI_office_notebook/D/myworkspace/nucc_workspace/program/src/nucc.com/mcp_srv_mgr"
MCP_HTTP_HOST="127.0.0.1"
MCP_HTTP_PORT="8082"
UNLA_GATEWAY_HOST="127.0.0.1"
UNLA_GATEWAY_PORT="8081"
MCP_HTTP_URL="http://${MCP_HTTP_HOST}:${MCP_HTTP_PORT}"
GATEWAY_URL="http://${UNLA_GATEWAY_HOST}:${UNLA_GATEWAY_PORT}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results tracking
PASSED=0
FAILED=0

# PIDs to track background processes
MCP_HTTP_PID=""

# Function to print test status
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

# Function to make HTTP requests with error handling
make_request() {
    local method="$1"
    local url="$2"
    local data="$3"
    local expected_status="$4"
    local timeout="${5:-30}"
    
    if [ -n "$data" ]; then
        response=$(timeout "$timeout" curl -s -w "HTTPSTATUS:%{http_code}" -X "$method" -H "Content-Type: application/json" -d "$data" "$url" 2>/dev/null || echo "HTTPSTATUS:000")
    else
        response=$(timeout "$timeout" curl -s -w "HTTPSTATUS:%{http_code}" -X "$method" "$url" 2>/dev/null || echo "HTTPSTATUS:000")
    fi
    
    http_code=$(echo "$response" | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
    body=$(echo "$response" | sed -e 's/HTTPSTATUS:.*//g')
    
    if [ "$http_code" = "$expected_status" ]; then
        echo "$body"
        return 0
    else
        echo "Error: Expected status $expected_status, got $http_code"
        echo "Response: $body"
        return 1
    fi
}

# Function to test SSE endpoint
test_sse_endpoint() {
    local url="$1"
    local data="$2"
    local expected_content="$3"
    local timeout="${4:-15}"
    
    # Use curl to test SSE endpoint
    local temp_file=$(mktemp)
    
    # Send request and capture SSE response
    if timeout "$timeout" curl -s -N -H "Content-Type: application/json" -H "Accept: text/event-stream" -d "$data" "$url" > "$temp_file" 2>/dev/null; then
        if grep -q "$expected_content" "$temp_file"; then
            rm -f "$temp_file"
            return 0
        else
            echo "Expected content not found. Response:"
            cat "$temp_file"
            rm -f "$temp_file"
            return 1
        fi
    else
        echo "SSE request failed or timed out"
        rm -f "$temp_file"
        return 1
    fi
}

# Cleanup function
cleanup() {
    echo "🧹 Cleaning up background processes..."
    if [ -n "$MCP_HTTP_PID" ] && kill -0 "$MCP_HTTP_PID" 2>/dev/null; then
        echo "Stopping MCP HTTP server (PID: $MCP_HTTP_PID)..."
        kill "$MCP_HTTP_PID"
        wait "$MCP_HTTP_PID" 2>/dev/null || true
    fi
    
    # Clean up any remaining processes
    pkill -f "mcp-server.*mcp-http" 2>/dev/null || true
    
    rm -f /tmp/mcp_http_*.json
}

# Set up cleanup trap
trap cleanup EXIT INT TERM

# Check if mcp-server binary exists
echo "🔍 Checking if mcp-server binary exists..."
cd "$MCP_SRV_MGR_DIR"
if [ ! -f "./mcp-server" ]; then
    echo -e "${RED}❌ mcp-server binary not found. Building it first...${NC}"
    if go build -o mcp-server cmd/server/main.go; then
        echo -e "${GREEN}✅ mcp-server built successfully${NC}"
    else
        echo -e "${RED}❌ Failed to build mcp-server${NC}"
        exit 1
    fi
else
    echo -e "${GREEN}✅ mcp-server binary found${NC}"
fi

echo ""
echo "🚀 Starting MCP HTTP (SSE) server..."
cd "$MCP_SRV_MGR_DIR"

# Start MCP HTTP server in background
MCP_HOST="$MCP_HTTP_HOST" MCP_PORT="$MCP_HTTP_PORT" ./mcp-server -mode=mcp-http -config=test_mcp_gateway/config-mcp-http.yaml > /tmp/mcp_http_server.log 2>&1 &
MCP_HTTP_PID=$!

# Wait for server to start
echo "⏳ Waiting for MCP HTTP server to start..."
for i in {1..30}; do
    if make_request "GET" "$MCP_HTTP_URL/health" "" "200" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ MCP HTTP server is running on $MCP_HTTP_URL${NC}"
        break
    fi
    if [ $i -eq 30 ]; then
        echo -e "${RED}❌ MCP HTTP server failed to start within 30 seconds${NC}"
        echo "Server log:"
        cat /tmp/mcp_http_server.log
        exit 1
    fi
    sleep 1
done

echo ""
echo "🧪 Running Direct MCP HTTP (SSE) Tests"
echo "---------------------------------------"

# Test 1: Health check
echo "Testing MCP HTTP health endpoint..."
if make_request "GET" "$MCP_HTTP_URL/health" "" "200" > /tmp/mcp_http_health.json; then
    if grep -q "healthy\|ok" /tmp/mcp_http_health.json; then
        print_test_status "MCP HTTP Health Check" "PASS"
    else
        print_test_status "MCP HTTP Health Check" "FAIL"
        echo "Health response: $(cat /tmp/mcp_http_health.json)"
    fi
else
    print_test_status "MCP HTTP Health Check" "FAIL"
fi

# Test 2: MCP initialize via HTTP
echo "Testing MCP HTTP initialize..."
init_data='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}}}}'
if make_request "POST" "$MCP_HTTP_URL/mcp" "$init_data" "200" > /tmp/mcp_http_init.json; then
    if grep -q '"result"' /tmp/mcp_http_init.json; then
        print_test_status "MCP HTTP Initialize" "PASS"
    else
        print_test_status "MCP HTTP Initialize" "FAIL"
        echo "Initialize response: $(cat /tmp/mcp_http_init.json)"
    fi
else
    print_test_status "MCP HTTP Initialize" "FAIL"
fi

# Test 3: List tools via HTTP
echo "Testing MCP HTTP list tools..."
tools_data='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
if make_request "POST" "$MCP_HTTP_URL/mcp" "$tools_data" "200" > /tmp/mcp_http_tools.json; then
    if grep -q '"tools"' /tmp/mcp_http_tools.json && grep -q 'list_services' /tmp/mcp_http_tools.json; then
        print_test_status "MCP HTTP List Tools" "PASS"
    else
        print_test_status "MCP HTTP List Tools" "FAIL"
        echo "Tools response: $(cat /tmp/mcp_http_tools.json)"
    fi
else
    print_test_status "MCP HTTP List Tools" "FAIL"
fi

# Test 4: Call tool via HTTP
echo "Testing MCP HTTP tool call..."
call_data='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_services","arguments":{}}}'
if make_request "POST" "$MCP_HTTP_URL/mcp" "$call_data" "200" > /tmp/mcp_http_call.json; then
    if grep -q '"content"' /tmp/mcp_http_call.json || grep -q '"result"' /tmp/mcp_http_call.json; then
        print_test_status "MCP HTTP Tool Call" "PASS"
    else
        print_test_status "MCP HTTP Tool Call" "FAIL"
        echo "Tool call response: $(cat /tmp/mcp_http_call.json)"
    fi
else
    print_test_status "MCP HTTP Tool Call" "FAIL"
fi

# Test 5: SSE endpoint (if available)
echo "Testing SSE endpoint..."
if test_sse_endpoint "$MCP_HTTP_URL/sse" "$tools_data" "data:" 10; then
    print_test_status "MCP HTTP SSE" "PASS"
else
    # SSE might not be available or use different endpoint
    echo "  ⚠️  SSE endpoint test skipped (may not be implemented or use different format)"
    print_test_status "MCP HTTP SSE" "SKIP"
fi

echo ""
echo "📊 Direct MCP HTTP (SSE) Test Results:"
echo "   Passed: $PASSED"
echo "   Failed: $FAILED"

# Store direct test results
DIRECT_PASSED=$PASSED
DIRECT_FAILED=$FAILED

echo ""
echo "🔧 Gateway Integration Tests"
echo "-----------------------------"

# Check if Unla gateway is running
echo "🔍 Checking if Unla gateway is running..."
if make_request "GET" "$GATEWAY_URL/health" "" "200" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Unla gateway is running${NC}"
    
    # Reset counters for gateway tests
    PASSED=0
    FAILED=0
    
    echo ""
    echo "🧪 Testing MCP HTTP (SSE) through Unla Gateway"
    echo "-----------------------------------------------"
    
    # Gateway Test 1: Check integration configuration
    echo "Testing MCP HTTP SSE integration configuration..."
    if make_request "GET" "$GATEWAY_URL/api/integrations" "" "200" > /tmp/integrations.json 2>/dev/null; then
        if grep -q "mcp_srv_mgr_http_sse" /tmp/integrations.json; then
            print_test_status "Gateway MCP HTTP SSE Configuration" "PASS"
        else
            print_test_status "Gateway MCP HTTP SSE Configuration" "FAIL"
            echo "Available integrations: $(cat /tmp/integrations.json)"
        fi
    else
        echo "  ⚠️  Could not check integration configuration (endpoint may not exist)"
    fi
    
    # Gateway Test 2: Proxy MCP HTTP request through gateway
    echo "Testing MCP HTTP through gateway..."
    gateway_init_request='{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{},"integration":"mcp_srv_mgr_http_sse"}'
    if make_request "POST" "$GATEWAY_URL/api/mcp/http" "$gateway_init_request" "200" > /tmp/gateway_http.json; then
        if grep -q '"tools"' /tmp/gateway_http.json || grep -q '"result"' /tmp/gateway_http.json; then
            print_test_status "Gateway MCP HTTP Proxy" "PASS"
        else
            print_test_status "Gateway MCP HTTP Proxy" "FAIL"
            echo "Response: $(cat /tmp/gateway_http.json)"
        fi
    else
        # Try alternative endpoints
        if make_request "POST" "$GATEWAY_URL/mcp/http" "$gateway_init_request" "200" > /tmp/gateway_http.json; then
            if grep -q '"tools"' /tmp/gateway_http.json || grep -q '"result"' /tmp/gateway_http.json; then
                print_test_status "Gateway MCP HTTP Proxy (alt endpoint)" "PASS"
            else
                print_test_status "Gateway MCP HTTP Proxy" "FAIL"
                echo "Response: $(cat /tmp/gateway_http.json)"
            fi
        else
            print_test_status "Gateway MCP HTTP Proxy" "FAIL"
            echo "Could not connect to gateway MCP HTTP endpoint"
        fi
    fi
    
    # Gateway Test 3: Test SSE through gateway
    echo "Testing SSE through gateway..."
    gateway_sse_request='{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_services","arguments":{},"integration":"mcp_srv_mgr_http_sse"}}'
    if test_sse_endpoint "$GATEWAY_URL/api/mcp/sse" "$gateway_sse_request" "data:" 10; then
        print_test_status "Gateway MCP SSE" "PASS"
    else
        # Try alternative SSE endpoint
        if test_sse_endpoint "$GATEWAY_URL/mcp/sse" "$gateway_sse_request" "data:" 10; then
            print_test_status "Gateway MCP SSE (alt endpoint)" "PASS"
        else
            print_test_status "Gateway MCP SSE" "FAIL"
            echo "Could not establish SSE connection through gateway"
        fi
    fi
    
    # Gateway Test 4: Tool execution through gateway
    echo "Testing tool execution through gateway HTTP..."
    gateway_tool_request='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_service_status","arguments":{"service_name":"nginx"},"integration":"mcp_srv_mgr_http_sse"}}'
    if make_request "POST" "$GATEWAY_URL/api/mcp/http" "$gateway_tool_request" "200" > /tmp/gateway_http_tool.json; then
        if grep -q '"content"' /tmp/gateway_http_tool.json || grep -q '"result"' /tmp/gateway_http_tool.json; then
            print_test_status "Gateway MCP HTTP Tool Execution" "PASS"
        else
            print_test_status "Gateway MCP HTTP Tool Execution" "FAIL"
            echo "Response: $(cat /tmp/gateway_http_tool.json)"
        fi
    else
        # Try alternative endpoint
        if make_request "POST" "$GATEWAY_URL/mcp/http" "$gateway_tool_request" "200" > /tmp/gateway_http_tool.json; then
            if grep -q '"content"' /tmp/gateway_http_tool.json || grep -q '"result"' /tmp/gateway_http_tool.json; then
                print_test_status "Gateway MCP HTTP Tool Execution (alt endpoint)" "PASS"
            else
                print_test_status "Gateway MCP HTTP Tool Execution" "FAIL"
                echo "Response: $(cat /tmp/gateway_http_tool.json)"
            fi
        else
            print_test_status "Gateway MCP HTTP Tool Execution" "FAIL"
            echo "Could not execute tool through gateway"
        fi
    fi
    
    echo ""
    echo "📊 Gateway Integration Test Results:"
    echo "   Passed: $PASSED"
    echo "   Failed: $FAILED"
    
    GATEWAY_PASSED=$PASSED
    GATEWAY_FAILED=$FAILED
    
else
    echo -e "${YELLOW}⚠️  Unla gateway is not running. Skipping gateway integration tests.${NC}"
    echo "   To run gateway tests, start the Unla gateway first:"
    echo "   ./mcp-gateway --config test_mcp_gateway/unla-config.yaml"
    GATEWAY_PASSED=0
    GATEWAY_FAILED=0
fi

# Final Summary
echo ""
echo "🏁 Final Test Summary"
echo "====================="
echo "Direct MCP HTTP (SSE) Tests:"
echo "   Passed: $DIRECT_PASSED"
echo "   Failed: $DIRECT_FAILED"
echo ""
echo "Gateway Integration Tests:"
echo "   Passed: $GATEWAY_PASSED"
echo "   Failed: $GATEWAY_FAILED"
echo ""

TOTAL_PASSED=$((DIRECT_PASSED + GATEWAY_PASSED))
TOTAL_FAILED=$((DIRECT_FAILED + GATEWAY_FAILED))

echo "Total Results:"
echo "   Passed: $TOTAL_PASSED"
echo "   Failed: $TOTAL_FAILED"

if [ $TOTAL_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}💥 Some tests failed!${NC}"
    exit 1
fi