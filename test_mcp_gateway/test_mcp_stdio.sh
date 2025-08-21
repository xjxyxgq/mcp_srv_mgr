#!/bin/bash

# Test MCP stdio protocol integration with Unla Gateway
# This script tests the MCP stdio protocol through the Unla gateway

set -e

echo "🧪 Testing MCP stdio Integration with Unla Gateway"
echo "=================================================="

# Configuration
MCP_SRV_MGR_DIR="/Users/xuguoqiang/SynologyDrive/Backup/MI_office_notebook/D/myworkspace/nucc_workspace/program/src/nucc.com/mcp_srv_mgr"
UNLA_GATEWAY_HOST="127.0.0.1"
UNLA_GATEWAY_PORT="8081"
GATEWAY_URL="http://${UNLA_GATEWAY_HOST}:${UNLA_GATEWAY_PORT}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results tracking
PASSED=0
FAILED=0

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

# Function to send MCP messages via stdio
send_mcp_message() {
    local message="$1"
    local expected_response="$2"
    local timeout="${3:-10}"
    
    # Create a temporary directory for this test
    local temp_dir=$(mktemp -d)
    local input_file="$temp_dir/input.json"
    local output_file="$temp_dir/output.json"
    
    echo "$message" > "$input_file"
    
    # Run mcp-server and capture output
    cd "$MCP_SRV_MGR_DIR"
    
    if timeout "$timeout" ./mcp-server -mode=mcp < "$input_file" > "$output_file" 2>/dev/null; then
        if [ -n "$expected_response" ]; then
            if grep -q "$expected_response" "$output_file"; then
                rm -rf "$temp_dir"
                return 0
            else
                echo "Expected response not found. Got:"
                cat "$output_file"
                rm -rf "$temp_dir"
                return 1
            fi
        else
            rm -rf "$temp_dir"
            return 0
        fi
    else
        echo "Command timed out or failed"
        rm -rf "$temp_dir"
        return 1
    fi
}

# Function to make HTTP requests with error handling
make_request() {
    local method="$1"
    local url="$2"
    local data="$3"
    local expected_status="$4"
    
    if [ -n "$data" ]; then
        response=$(curl -s -w "HTTPSTATUS:%{http_code}" -X "$method" -H "Content-Type: application/json" -d "$data" "$url" 2>/dev/null || echo "HTTPSTATUS:000")
    else
        response=$(curl -s -w "HTTPSTATUS:%{http_code}" -X "$method" "$url" 2>/dev/null || echo "HTTPSTATUS:000")
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
echo "🧪 Running Direct MCP stdio Tests"
echo "----------------------------------"

# Test 1: Initialize MCP session
echo "Testing MCP initialization..."
init_message='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true},"sampling":{}}}}'
if send_mcp_message "$init_message" '"result"'; then
    print_test_status "MCP Initialize" "PASS"
else
    print_test_status "MCP Initialize" "FAIL"
fi

# Test 2: List tools
echo "Testing MCP list tools..."
list_tools_message='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
if send_mcp_message "$list_tools_message" '"tools"'; then
    print_test_status "MCP List Tools" "PASS"
else
    print_test_status "MCP List Tools" "FAIL"
fi

# Test 3: Call list_services tool
echo "Testing list_services tool..."
call_tool_message='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_services","arguments":{}}}'
if send_mcp_message "$call_tool_message" '"content"'; then
    print_test_status "MCP Call List Services Tool" "PASS"
else
    print_test_status "MCP Call List Services Tool" "FAIL"
fi

# Test 4: Call get_service_status tool
echo "Testing get_service_status tool..."
status_tool_message='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_service_status","arguments":{"service_name":"nginx"}}}'
if send_mcp_message "$status_tool_message" '"content"'; then
    print_test_status "MCP Get Service Status Tool" "PASS"
else
    print_test_status "MCP Get Service Status Tool" "FAIL"
fi

# Test 5: List prompts
echo "Testing MCP list prompts..."
list_prompts_message='{"jsonrpc":"2.0","id":5,"method":"prompts/list","params":{}}'
if send_mcp_message "$list_prompts_message" '"prompts"'; then
    print_test_status "MCP List Prompts" "PASS"
else
    print_test_status "MCP List Prompts" "FAIL"
fi

echo ""
echo "📊 Direct MCP stdio Test Results:"
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
    echo "🧪 Testing MCP stdio through Unla Gateway"
    echo "------------------------------------------"
    
    # Gateway Test 1: Check if MCP stdio integration is configured
    echo "Testing MCP stdio integration configuration..."
    if make_request "GET" "$GATEWAY_URL/api/integrations" "" "200" > /tmp/integrations.json 2>/dev/null; then
        if grep -q "mcp_srv_mgr_stdio" /tmp/integrations.json; then
            print_test_status "Gateway MCP stdio Configuration" "PASS"
        else
            print_test_status "Gateway MCP stdio Configuration" "FAIL"
            echo "Available integrations: $(cat /tmp/integrations.json)"
        fi
    else
        echo "  ⚠️  Could not check integration configuration (endpoint may not exist)"
        # This is acceptable as different versions of Unla might have different endpoints
    fi
    
    # Gateway Test 2: Test MCP stdio through gateway proxy
    echo "Testing MCP stdio through gateway proxy..."
    gateway_mcp_request='{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{},"integration":"mcp_srv_mgr_stdio"}'
    if make_request "POST" "$GATEWAY_URL/api/mcp/stdio" "$gateway_mcp_request" "200" > /tmp/gateway_stdio.json; then
        if grep -q '"tools"' /tmp/gateway_stdio.json || grep -q '"result"' /tmp/gateway_stdio.json; then
            print_test_status "Gateway MCP stdio Proxy" "PASS"
        else
            print_test_status "Gateway MCP stdio Proxy" "FAIL"
            echo "Response: $(cat /tmp/gateway_stdio.json)"
        fi
    else
        # Try alternative endpoint
        if make_request "POST" "$GATEWAY_URL/mcp/stdio" "$gateway_mcp_request" "200" > /tmp/gateway_stdio.json; then
            if grep -q '"tools"' /tmp/gateway_stdio.json || grep -q '"result"' /tmp/gateway_stdio.json; then
                print_test_status "Gateway MCP stdio Proxy (alt endpoint)" "PASS"
            else
                print_test_status "Gateway MCP stdio Proxy" "FAIL"
                echo "Response: $(cat /tmp/gateway_stdio.json)"
            fi
        else
            print_test_status "Gateway MCP stdio Proxy" "FAIL"
            echo "Could not connect to gateway MCP stdio endpoint"
        fi
    fi
    
    # Gateway Test 3: Test tool execution through gateway
    echo "Testing tool execution through gateway..."
    gateway_tool_request='{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_services","arguments":{},"integration":"mcp_srv_mgr_stdio"}}'
    if make_request "POST" "$GATEWAY_URL/api/mcp/stdio" "$gateway_tool_request" "200" > /tmp/gateway_tool.json; then
        if grep -q '"content"' /tmp/gateway_tool.json || grep -q '"result"' /tmp/gateway_tool.json; then
            print_test_status "Gateway MCP stdio Tool Execution" "PASS"
        else
            print_test_status "Gateway MCP stdio Tool Execution" "FAIL"
            echo "Response: $(cat /tmp/gateway_tool.json)"
        fi
    else
        # Try alternative endpoint
        if make_request "POST" "$GATEWAY_URL/mcp/stdio" "$gateway_tool_request" "200" > /tmp/gateway_tool.json; then
            if grep -q '"content"' /tmp/gateway_tool.json || grep -q '"result"' /tmp/gateway_tool.json; then
                print_test_status "Gateway MCP stdio Tool Execution (alt endpoint)" "PASS"
            else
                print_test_status "Gateway MCP stdio Tool Execution" "FAIL"
                echo "Response: $(cat /tmp/gateway_tool.json)"
            fi
        else
            print_test_status "Gateway MCP stdio Tool Execution" "FAIL"
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
echo "Direct MCP stdio Tests:"
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

# Cleanup temp files
rm -f /tmp/integrations.json /tmp/gateway_stdio.json /tmp/gateway_tool.json

if [ $TOTAL_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}💥 Some tests failed!${NC}"
    exit 1
fi