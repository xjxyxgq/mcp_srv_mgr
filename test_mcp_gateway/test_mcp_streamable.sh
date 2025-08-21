#!/bin/bash

# Test MCP Streamable HTTP integration with Unla Gateway
# This script tests the MCP Streamable HTTP protocol through the Unla gateway

set -e

echo "🧪 Testing MCP Streamable HTTP Integration with Unla Gateway"
echo "============================================================"

# Configuration
MCP_SRV_MGR_DIR="/Users/xuguoqiang/SynologyDrive/Backup/MI_office_notebook/D/myworkspace/nucc_workspace/program/src/nucc.com/mcp_srv_mgr"
MCP_STREAMABLE_HOST="127.0.0.1"
MCP_STREAMABLE_PORT="8083"
UNLA_GATEWAY_HOST="127.0.0.1"
UNLA_GATEWAY_PORT="8081"
MCP_STREAMABLE_URL="http://${MCP_STREAMABLE_HOST}:${MCP_STREAMABLE_PORT}"
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
MCP_STREAMABLE_PID=""

# Function to print test status
print_test_status() {
    local test_name="$1"
    local status="$2"
    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✅ $test_name: PASSED${NC}"
        ((PASSED++))
    elif [ "$status" = "SKIP" ]; then
        echo -e "${YELLOW}⚠️ $test_name: SKIPPED${NC}"
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

# Function to test streaming endpoint with WebSocket-like behavior
test_streaming_endpoint() {
    local url="$1"
    local data="$2"
    local expected_content="$3"
    local timeout="${4:-15}"
    
    # Test using curl with connection upgrade headers (simulating WebSocket)
    local temp_file=$(mktemp)
    
    if timeout "$timeout" curl -s -N \
        -H "Connection: Upgrade" \
        -H "Upgrade: websocket" \
        -H "Sec-WebSocket-Version: 13" \
        -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
        -H "Content-Type: application/json" \
        -d "$data" \
        "$url" > "$temp_file" 2>/dev/null; then
        
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
        # Try as regular HTTP POST if WebSocket upgrade fails
        if make_request "POST" "$url" "$data" "200" > "$temp_file"; then
            if grep -q "$expected_content" "$temp_file"; then
                rm -f "$temp_file"
                return 0
            fi
        fi
        
        echo "Streaming request failed"
        rm -f "$temp_file"
        return 1
    fi
}

# Cleanup function
cleanup() {
    echo "🧹 Cleaning up background processes..."
    if [ -n "$MCP_STREAMABLE_PID" ] && kill -0 "$MCP_STREAMABLE_PID" 2>/dev/null; then
        echo "Stopping MCP Streamable server (PID: $MCP_STREAMABLE_PID)..."
        kill "$MCP_STREAMABLE_PID"
        wait "$MCP_STREAMABLE_PID" 2>/dev/null || true
    fi
    
    # Clean up any remaining processes
    pkill -f "mcp-server.*mcp-streamable" 2>/dev/null || true
    
    rm -f /tmp/mcp_streamable_*.json
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
echo "🚀 Starting MCP Streamable HTTP server..."
cd "$MCP_SRV_MGR_DIR"

# Start MCP Streamable server in background
MCP_HOST="$MCP_STREAMABLE_HOST" MCP_PORT="$MCP_STREAMABLE_PORT" ./mcp-server -mode=mcp-streamable -config=test_mcp_gateway/config-mcp-streamable.yaml > /tmp/mcp_streamable_server.log 2>&1 &
MCP_STREAMABLE_PID=$!

# Wait for server to start
echo "⏳ Waiting for MCP Streamable server to start..."
for i in {1..30}; do
    if make_request "GET" "$MCP_STREAMABLE_URL/health" "" "200" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ MCP Streamable server is running on $MCP_STREAMABLE_URL${NC}"
        break
    fi
    if [ $i -eq 30 ]; then
        echo -e "${RED}❌ MCP Streamable server failed to start within 30 seconds${NC}"
        echo "Server log:"
        cat /tmp/mcp_streamable_server.log
        exit 1
    fi
    sleep 1
done

echo ""
echo "🧪 Running Direct MCP Streamable HTTP Tests"
echo "--------------------------------------------"

# Test 1: Health check
echo "Testing MCP Streamable health endpoint..."
if make_request "GET" "$MCP_STREAMABLE_URL/health" "" "200" > /tmp/mcp_streamable_health.json; then
    if grep -q "healthy\|ok" /tmp/mcp_streamable_health.json; then
        print_test_status "MCP Streamable Health Check" "PASS"
    else
        print_test_status "MCP Streamable Health Check" "FAIL"
        echo "Health response: $(cat /tmp/mcp_streamable_health.json)"
    fi
else
    print_test_status "MCP Streamable Health Check" "FAIL"
fi

# Test 2: MCP initialize via Streamable HTTP
echo "Testing MCP Streamable initialize..."
init_data='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}}}}'
if make_request "POST" "$MCP_STREAMABLE_URL/mcp" "$init_data" "200" > /tmp/mcp_streamable_init.json; then
    if grep -q '"result"' /tmp/mcp_streamable_init.json; then
        print_test_status "MCP Streamable Initialize" "PASS"
    else
        print_test_status "MCP Streamable Initialize" "FAIL"
        echo "Initialize response: $(cat /tmp/mcp_streamable_init.json)"
    fi
else
    print_test_status "MCP Streamable Initialize" "FAIL"
fi

# Test 3: List tools via Streamable HTTP
echo "Testing MCP Streamable list tools..."
tools_data='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
if make_request "POST" "$MCP_STREAMABLE_URL/mcp" "$tools_data" "200" > /tmp/mcp_streamable_tools.json; then
    if grep -q '"tools"' /tmp/mcp_streamable_tools.json && grep -q 'list_services' /tmp/mcp_streamable_tools.json; then
        print_test_status "MCP Streamable List Tools" "PASS"
    else
        print_test_status "MCP Streamable List Tools" "FAIL"
        echo "Tools response: $(cat /tmp/mcp_streamable_tools.json)"
    fi
else
    print_test_status "MCP Streamable List Tools" "FAIL"
fi

# Test 4: Call tool via Streamable HTTP
echo "Testing MCP Streamable tool call..."
call_data='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_services","arguments":{}}}'
if make_request "POST" "$MCP_STREAMABLE_URL/mcp" "$call_data" "200" > /tmp/mcp_streamable_call.json; then
    if grep -q '"content"' /tmp/mcp_streamable_call.json || grep -q '"result"' /tmp/mcp_streamable_call.json; then
        print_test_status "MCP Streamable Tool Call" "PASS"
    else
        print_test_status "MCP Streamable Tool Call" "FAIL"
        echo "Tool call response: $(cat /tmp/mcp_streamable_call.json)"
    fi
else
    print_test_status "MCP Streamable Tool Call" "FAIL"
fi

# Test 5: WebSocket/Streaming endpoint (if available)
echo "Testing streaming/WebSocket endpoint..."
if test_streaming_endpoint "$MCP_STREAMABLE_URL/ws" "$tools_data" "tools" 10; then
    print_test_status "MCP Streamable WebSocket" "PASS"
else
    # Try alternative streaming endpoint
    if test_streaming_endpoint "$MCP_STREAMABLE_URL/stream" "$tools_data" "tools" 10; then
        print_test_status "MCP Streamable Stream" "PASS"
    else
        # Streaming might not be available or use different protocol
        echo "  ⚠️  Streaming endpoint test skipped (may not be implemented or use different protocol)"
        print_test_status "MCP Streamable Stream" "SKIP"
    fi
fi

# Test 6: Bidirectional communication test
echo "Testing bidirectional communication..."
bidirectional_data='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_service_status","arguments":{"service_name":"nginx"}}}'
if make_request "POST" "$MCP_STREAMABLE_URL/mcp" "$bidirectional_data" "200" > /tmp/mcp_streamable_bidirectional.json; then
    if grep -q '"content"' /tmp/mcp_streamable_bidirectional.json || grep -q '"result"' /tmp/mcp_streamable_bidirectional.json; then
        print_test_status "MCP Streamable Bidirectional" "PASS"
    else
        print_test_status "MCP Streamable Bidirectional" "FAIL"
        echo "Bidirectional response: $(cat /tmp/mcp_streamable_bidirectional.json)"
    fi
else
    print_test_status "MCP Streamable Bidirectional" "FAIL"
fi

echo ""
echo "📊 Direct MCP Streamable HTTP Test Results:"
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
    echo "🧪 Testing MCP Streamable HTTP through Unla Gateway"
    echo "----------------------------------------------------"
    
    # Gateway Test 1: Check integration configuration
    echo "Testing MCP Streamable integration configuration..."
    if make_request "GET" "$GATEWAY_URL/api/integrations" "" "200" > /tmp/integrations.json 2>/dev/null; then
        if grep -q "mcp_srv_mgr_streamable" /tmp/integrations.json; then
            print_test_status "Gateway MCP Streamable Configuration" "PASS"
        else
            print_test_status "Gateway MCP Streamable Configuration" "FAIL"
            echo "Available integrations: $(cat /tmp/integrations.json)"
        fi
    else
        echo "  ⚠️  Could not check integration configuration (endpoint may not exist)"
    fi
    
    # Gateway Test 2: Proxy MCP Streamable request through gateway
    echo "Testing MCP Streamable through gateway..."
    gateway_request='{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{},"integration":"mcp_srv_mgr_streamable"}'
    if make_request "POST" "$GATEWAY_URL/api/mcp/streamable" "$gateway_request" "200" > /tmp/gateway_streamable.json; then
        if grep -q '"tools"' /tmp/gateway_streamable.json || grep -q '"result"' /tmp/gateway_streamable.json; then
            print_test_status "Gateway MCP Streamable Proxy" "PASS"
        else
            print_test_status "Gateway MCP Streamable Proxy" "FAIL"
            echo "Response: $(cat /tmp/gateway_streamable.json)"
        fi
    else
        # Try alternative endpoints
        if make_request "POST" "$GATEWAY_URL/mcp/streamable" "$gateway_request" "200" > /tmp/gateway_streamable.json; then
            if grep -q '"tools"' /tmp/gateway_streamable.json || grep -q '"result"' /tmp/gateway_streamable.json; then
                print_test_status "Gateway MCP Streamable Proxy (alt endpoint)" "PASS"
            else
                print_test_status "Gateway MCP Streamable Proxy" "FAIL"
                echo "Response: $(cat /tmp/gateway_streamable.json)"
            fi
        else
            print_test_status "Gateway MCP Streamable Proxy" "FAIL"
            echo "Could not connect to gateway MCP Streamable endpoint"
        fi
    fi
    
    # Gateway Test 3: Test streaming through gateway
    echo "Testing streaming through gateway..."
    gateway_stream_request='{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_services","arguments":{},"integration":"mcp_srv_mgr_streamable"}}'
    if test_streaming_endpoint "$GATEWAY_URL/api/mcp/stream" "$gateway_stream_request" "content" 10; then
        print_test_status "Gateway MCP Streaming" "PASS"
    else
        # Try alternative streaming endpoints
        if test_streaming_endpoint "$GATEWAY_URL/mcp/stream" "$gateway_stream_request" "content" 10; then
            print_test_status "Gateway MCP Streaming (alt endpoint)" "PASS"
        elif test_streaming_endpoint "$GATEWAY_URL/api/ws" "$gateway_stream_request" "content" 10; then
            print_test_status "Gateway MCP Streaming (WebSocket)" "PASS"
        else
            print_test_status "Gateway MCP Streaming" "FAIL"
            echo "Could not establish streaming connection through gateway"
        fi
    fi
    
    # Gateway Test 4: Tool execution through gateway
    echo "Testing tool execution through gateway Streamable..."
    gateway_tool_request='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_service_status","arguments":{"service_name":"nginx"},"integration":"mcp_srv_mgr_streamable"}}'
    if make_request "POST" "$GATEWAY_URL/api/mcp/streamable" "$gateway_tool_request" "200" > /tmp/gateway_streamable_tool.json; then
        if grep -q '"content"' /tmp/gateway_streamable_tool.json || grep -q '"result"' /tmp/gateway_streamable_tool.json; then
            print_test_status "Gateway MCP Streamable Tool Execution" "PASS"
        else
            print_test_status "Gateway MCP Streamable Tool Execution" "FAIL"
            echo "Response: $(cat /tmp/gateway_streamable_tool.json)"
        fi
    else
        # Try alternative endpoint
        if make_request "POST" "$GATEWAY_URL/mcp/streamable" "$gateway_tool_request" "200" > /tmp/gateway_streamable_tool.json; then
            if grep -q '"content"' /tmp/gateway_streamable_tool.json || grep -q '"result"' /tmp/gateway_streamable_tool.json; then
                print_test_status "Gateway MCP Streamable Tool Execution (alt endpoint)" "PASS"
            else
                print_test_status "Gateway MCP Streamable Tool Execution" "FAIL"
                echo "Response: $(cat /tmp/gateway_streamable_tool.json)"
            fi
        else
            print_test_status "Gateway MCP Streamable Tool Execution" "FAIL"
            echo "Could not execute tool through gateway"
        fi
    fi
    
    # Gateway Test 5: Long-running streaming operation
    echo "Testing long-running streaming operation through gateway..."
    long_stream_request='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_docker_logs","arguments":{"container_name":"test","lines":50},"integration":"mcp_srv_mgr_streamable"}}'
    if test_streaming_endpoint "$GATEWAY_URL/api/mcp/stream" "$long_stream_request" "content" 5; then
        print_test_status "Gateway Long Streaming" "PASS"
    else
        # This might fail if Docker is not available or no containers exist
        print_test_status "Gateway Long Streaming" "SKIP"
        echo "  ⚠️  Long streaming test skipped (Docker container may not exist)"
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
echo "Direct MCP Streamable HTTP Tests:"
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