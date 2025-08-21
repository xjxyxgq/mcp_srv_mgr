#!/bin/bash

# Test HTTP API Integration with Unla Gateway
# This script tests the HTTP REST API endpoints through the Unla gateway

set -e

echo "🧪 Testing HTTP API Integration with Unla Gateway"
echo "=================================================="

# Configuration
MCP_SRV_MGR_HOST="127.0.0.1"
MCP_SRV_MGR_PORT="8080"
UNLA_GATEWAY_HOST="127.0.0.1"
UNLA_GATEWAY_PORT="8081"
BASE_URL="http://${MCP_SRV_MGR_HOST}:${MCP_SRV_MGR_PORT}"
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

# PIDs tracking for cleanup
MCP_SRV_MGR_PID=""
MCP_GATEWAY_PID=""

# Cleanup function
cleanup_services() {
    if [ -n "$MCP_SRV_MGR_PID" ]; then
        kill "$MCP_SRV_MGR_PID" 2>/dev/null || true
        echo "Stopped mcp_srv_mgr (PID: $MCP_SRV_MGR_PID)"
    fi
    if [ -n "$MCP_GATEWAY_PID" ]; then
        kill "$MCP_GATEWAY_PID" 2>/dev/null || true
        echo "Stopped mcp-gateway (PID: $MCP_GATEWAY_PID)"
    fi
    # Clean up any remaining processes
    pkill -f "mcp-server.*-mode=http" 2>/dev/null || true
}

trap cleanup_services EXIT

# Function to wait for service to be ready
wait_for_service() {
    local url="$1"
    local service_name="$2"
    local max_attempts=15
    local attempt=1
    
    echo "⏳ Waiting for $service_name to be ready..."
    
    while [ $attempt -le $max_attempts ]; do
        if curl -f -s "$url" > /dev/null 2>&1; then
            echo -e "${GREEN}✅ $service_name is ready!${NC}"
            return 0
        fi
        sleep 1
        attempt=$((attempt + 1))
    done
    
    echo -e "${RED}❌ $service_name failed to start after $max_attempts attempts${NC}"
    return 1
}

# Check if mcp-server is running, if not start it
echo "🔍 Checking if mcp_srv_mgr HTTP server is running..."
if make_request "GET" "$BASE_URL/health" "" "200" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ mcp_srv_mgr HTTP server is already running${NC}"
else
    echo "🚀 Starting mcp_srv_mgr HTTP server..."
    ./mcp-server -mode=http -config=test_mcp_gateway/config.yaml > /tmp/mcp_srv_mgr_test.log 2>&1 &
    MCP_SRV_MGR_PID=$!
    
    if wait_for_service "$BASE_URL/health" "mcp_srv_mgr HTTP server"; then
        echo -e "${GREEN}✅ mcp_srv_mgr HTTP server started successfully${NC}"
    else
        echo -e "${RED}❌ Failed to start mcp_srv_mgr HTTP server${NC}"
        echo "Log output:"
        tail -5 /tmp/mcp_srv_mgr_test.log 2>/dev/null || echo "No log available"
        exit 1
    fi
fi

echo ""
echo "🧪 Running Direct API Tests (before gateway integration)"
echo "--------------------------------------------------------"

# Test 1: Health Check
echo "Testing health endpoint..."
if make_request "GET" "$BASE_URL/health" "" "200" > /tmp/health_response.json; then
    if grep -q '"status":"healthy"' /tmp/health_response.json; then
        print_test_status "Health Check" "PASS"
    else
        print_test_status "Health Check" "FAIL"
        echo "Response: $(cat /tmp/health_response.json)"
    fi
else
    print_test_status "Health Check" "FAIL"
fi

# Test 2: List Services
echo "Testing list services endpoint..."
if make_request "GET" "$BASE_URL/services" "" "200" > /tmp/services_response.json; then
    if grep -q '"success":true' /tmp/services_response.json; then
        print_test_status "List Services" "PASS"
        echo "  Services found: $(jq -r '.services | length' /tmp/services_response.json 2>/dev/null || echo "N/A")"
    else
        print_test_status "List Services" "FAIL"
        echo "Response: $(cat /tmp/services_response.json)"
    fi
else
    print_test_status "List Services" "FAIL"
fi

# Test 3: Get Service Status (using a mock service that should exist)
echo "Testing get service status endpoint..."
if make_request "GET" "$BASE_URL/services/nginx/status" "" "200" > /tmp/status_response.json; then
    if grep -q '"success":true' /tmp/status_response.json || grep -q '"name":"nginx"' /tmp/status_response.json; then
        print_test_status "Get Service Status" "PASS"
    else
        print_test_status "Get Service Status" "FAIL"
        echo "Response: $(cat /tmp/status_response.json)"
    fi
else
    # Try with a systemd service type parameter
    if make_request "GET" "$BASE_URL/services/nginx/status?type=systemd" "" "200" > /tmp/status_response.json; then
        print_test_status "Get Service Status (with type)" "PASS"
    else
        print_test_status "Get Service Status" "FAIL"
    fi
fi

# Test 4: Service Action via generic endpoint
echo "Testing service action endpoint..."
service_action_data='{"name":"nginx","action":"status","type":"systemd"}'
if make_request "POST" "$BASE_URL/services/action" "$service_action_data" "200" > /tmp/action_response.json; then
    if grep -q '"success":true' /tmp/action_response.json; then
        print_test_status "Service Action" "PASS"
    else
        print_test_status "Service Action" "FAIL"
        echo "Response: $(cat /tmp/action_response.json)"
    fi
else
    print_test_status "Service Action" "FAIL"
fi

# Test 5: Docker Logs (if Docker is available)
echo "Testing Docker logs endpoint..."
if make_request "GET" "$BASE_URL/docker/test-container/logs" "" "200" > /tmp/docker_logs.json 2>/dev/null; then
    if grep -q '"success":true' /tmp/docker_logs.json; then
        print_test_status "Docker Logs" "PASS"
    else
        print_test_status "Docker Logs" "FAIL"
    fi
else
    echo "  ⚠️  Docker logs test skipped (container may not exist)"
fi

echo ""
echo "📊 Direct API Test Results:"
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
if make_request "GET" "$GATEWAY_URL/health_check" "" "200" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Unla gateway is already running${NC}"
    
    # Reset counters for gateway tests
    PASSED=0
    FAILED=0
    
    echo ""
    echo "🧪 Testing MCP Service Manager through Unla Gateway"
    echo "---------------------------------------------------"
    
    # Gateway Test 1: List tools (MCP protocol through gateway)
    echo "Testing MCP tools listing through gateway..."
    mcp_tools_request='{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
    if make_request "POST" "$GATEWAY_URL/mcp" "$mcp_tools_request" "200" > /tmp/gateway_tools.json; then
        if grep -q '"list_services"' /tmp/gateway_tools.json; then
            print_test_status "Gateway MCP Tools List" "PASS"
        else
            print_test_status "Gateway MCP Tools List" "FAIL"
            echo "Response: $(cat /tmp/gateway_tools.json)"
        fi
    else
        print_test_status "Gateway MCP Tools List" "FAIL"
    fi
    
    # Gateway Test 2: Call list_services through gateway
    echo "Testing list_services tool through gateway..."
    mcp_call_request='{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_services","arguments":{}}}'
    if make_request "POST" "$GATEWAY_URL/mcp" "$mcp_call_request" "200" > /tmp/gateway_list_services.json; then
        if grep -q '"result"' /tmp/gateway_list_services.json; then
            print_test_status "Gateway List Services Tool" "PASS"
        else
            print_test_status "Gateway List Services Tool" "FAIL"
            echo "Response: $(cat /tmp/gateway_list_services.json)"
        fi
    else
        print_test_status "Gateway List Services Tool" "FAIL"
    fi
    
    # Gateway Test 3: Call get_service_status through gateway
    echo "Testing get_service_status tool through gateway..."
    mcp_status_request='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_service_status","arguments":{"service_name":"nginx"}}}'
    if make_request "POST" "$GATEWAY_URL/mcp" "$mcp_status_request" "200" > /tmp/gateway_status.json; then
        if grep -q '"result"' /tmp/gateway_status.json; then
            print_test_status "Gateway Get Service Status Tool" "PASS"
        else
            print_test_status "Gateway Get Service Status Tool" "FAIL"
            echo "Response: $(cat /tmp/gateway_status.json)"
        fi
    else
        print_test_status "Gateway Get Service Status Tool" "FAIL"
    fi
    
    echo ""
    echo "📊 Gateway Integration Test Results:"
    echo "   Passed: $PASSED"
    echo "   Failed: $FAILED"
    
    GATEWAY_PASSED=$PASSED
    GATEWAY_FAILED=$FAILED
    
else
    echo "🚀 Starting Unla gateway..."
    ./mcp-gateway --conf test_mcp_gateway/mcp-gateway-working.yaml > /tmp/mcp_gateway_test.log 2>&1 &
    MCP_GATEWAY_PID=$!
    
    if wait_for_service "$GATEWAY_URL/health_check" "Unla gateway"; then
        echo -e "${GREEN}✅ Unla gateway started successfully${NC}"
        
        # Reset counters for gateway tests
        PASSED=0
        FAILED=0
        
        echo ""
        echo "🧪 Testing MCP Service Manager through Unla Gateway"
        echo "---------------------------------------------------"
        
        # Gateway Test 1: Health check
        echo "Testing gateway health check..."
        if make_request "GET" "$GATEWAY_URL/health_check" "" "200" > /tmp/gateway_health.json; then
            if grep -q '"status":"ok"' /tmp/gateway_health.json; then
                print_test_status "Gateway Health Check" "PASS"
            else
                print_test_status "Gateway Health Check" "FAIL"
            fi
        else
            print_test_status "Gateway Health Check" "FAIL"
        fi
        
        echo ""
        echo "📊 Gateway Integration Test Results:"
        echo "   Passed: $PASSED"
        echo "   Failed: $FAILED"
        
        GATEWAY_PASSED=$PASSED
        GATEWAY_FAILED=$FAILED
    else
        echo -e "${RED}❌ Failed to start Unla gateway${NC}"
        echo "Log output:"
        tail -5 /tmp/mcp_gateway_test.log 2>/dev/null || echo "No log available"
        GATEWAY_PASSED=0
        GATEWAY_FAILED=0
    fi
fi

# Final Summary
echo ""
echo "🏁 Final Test Summary"
echo "====================="
echo "Direct API Tests:"
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
rm -f /tmp/health_response.json /tmp/services_response.json /tmp/status_response.json
rm -f /tmp/action_response.json /tmp/docker_logs.json
rm -f /tmp/gateway_tools.json /tmp/gateway_list_services.json /tmp/gateway_status.json

if [ $TOTAL_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}💥 Some tests failed!${NC}"
    exit 1
fi