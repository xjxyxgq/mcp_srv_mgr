#!/bin/bash

# Test MySQL database integration for Unla Gateway
# This script tests the MySQL database setup and data persistence for the Unla gateway

set -e

echo "🧪 Testing MySQL Database Integration for Unla Gateway"
echo "======================================================"

# Configuration
MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3311"
MYSQL_USER="root"
MYSQL_PASS="nov24feb11"
MYSQL_DB="unla_gateway"
REDIS_HOST="127.0.0.1"
REDIS_PORT="6379"

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
    elif [ "$status" = "SKIP" ]; then
        echo -e "${YELLOW}⚠️ $test_name: SKIPPED${NC}"
    else
        echo -e "${RED}❌ $test_name: FAILED${NC}"
        ((FAILED++))
    fi
}

# Function to execute MySQL queries
mysql_query() {
    local query="$1"
    local silent="${2:-false}"
    
    if [ "$silent" = "true" ]; then
        mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" -D "$MYSQL_DB" -e "$query" 2>/dev/null
    else
        mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" -D "$MYSQL_DB" -e "$query"
    fi
}

# Function to check if MySQL is accessible
check_mysql_connection() {
    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" -e "SELECT 1;" > /dev/null 2>&1
}

# Function to check if Redis is accessible
check_redis_connection() {
    redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" ping > /dev/null 2>&1
}

# Function to test Redis operations
redis_command() {
    local command="$1"
    redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" $command 2>/dev/null
}

echo ""
echo "🔍 Checking Database Connectivity"
echo "----------------------------------"

# Test MySQL connection
echo "Testing MySQL connection..."
if check_mysql_connection; then
    print_test_status "MySQL Connection" "PASS"
    echo "  Connected to MySQL at $MYSQL_HOST:$MYSQL_PORT"
else
    print_test_status "MySQL Connection" "FAIL"
    echo -e "${RED}❌ Cannot connect to MySQL. Please ensure MySQL is running:${NC}"
    echo "   docker-compose -f test_mcp_gateway/docker-compose.yml up -d mysql"
    echo "   OR start your existing MySQL server on port $MYSQL_PORT"
    exit 1
fi

# Test Redis connection
echo "Testing Redis connection..."
if check_redis_connection; then
    print_test_status "Redis Connection" "PASS"
    echo "  Connected to Redis at $REDIS_HOST:$REDIS_PORT"
else
    echo -e "${YELLOW}⚠️  Redis connection failed. Starting with docker compose...${NC}"
    
    # Try to start Redis with docker compose (try both old and new syntax)
    DOCKER_COMPOSE_CMD=""
    if command -v "docker-compose" > /dev/null 2>&1; then
        DOCKER_COMPOSE_CMD="docker-compose"
    elif command -v "docker" > /dev/null 2>&1 && docker compose version > /dev/null 2>&1; then
        DOCKER_COMPOSE_CMD="docker compose"
    fi
    
    if [[ -n "$DOCKER_COMPOSE_CMD" ]] && $DOCKER_COMPOSE_CMD -f test_mcp_gateway/docker-compose.yml up -d redis 2>/dev/null; then
        echo "  Waiting for Redis to start..."
        sleep 5
        if check_redis_connection; then
            print_test_status "Redis Connection (after start)" "PASS"
        else
            print_test_status "Redis Connection" "FAIL"
            echo "  Redis tests will be skipped"
        fi
    else
        print_test_status "Redis Connection" "FAIL"
        echo "  Redis tests will be skipped"
    fi
fi

echo ""
echo "🗄️ Testing MySQL Database Setup"
echo "--------------------------------"

# Test 1: Database existence
echo "Testing database existence..."
if mysql_query "USE $MYSQL_DB;" true; then
    print_test_status "Database Exists" "PASS"
else
    echo "Creating database $MYSQL_DB..."
    if mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" -e "CREATE DATABASE IF NOT EXISTS $MYSQL_DB;" > /dev/null 2>&1; then
        print_test_status "Database Creation" "PASS"
    else
        print_test_status "Database Creation" "FAIL"
        exit 1
    fi
fi

# Test 2: Table creation from init.sql
echo "Testing table creation..."
if mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" -D "$MYSQL_DB" < test_mcp_gateway/init.sql > /dev/null 2>&1; then
    print_test_status "Table Creation" "PASS"
else
    print_test_status "Table Creation" "FAIL"
    echo "Failed to execute init.sql"
fi

# Test 3: Verify table structure
echo "Testing table structure..."
tables_exist=true

# Check unla_sessions table
if mysql_query "DESCRIBE unla_sessions;" true > /dev/null 2>&1; then
    echo "  ✅ unla_sessions table exists"
else
    echo "  ❌ unla_sessions table missing"
    tables_exist=false
fi

# Check unla_configurations table
if mysql_query "DESCRIBE unla_configurations;" true > /dev/null 2>&1; then
    echo "  ✅ unla_configurations table exists"
else
    echo "  ❌ unla_configurations table missing"
    tables_exist=false
fi

# Check unla_metrics table
if mysql_query "DESCRIBE unla_metrics;" true > /dev/null 2>&1; then
    echo "  ✅ unla_metrics table exists"
else
    echo "  ❌ unla_metrics table missing"
    tables_exist=false
fi

if [ "$tables_exist" = "true" ]; then
    print_test_status "Table Structure" "PASS"
else
    print_test_status "Table Structure" "FAIL"
fi

echo ""
echo "💾 Testing Data Operations"
echo "---------------------------"

# Test 4: Insert session data
echo "Testing session data insertion..."
session_id="test_session_$(date +%s)"
user_id="test_user"
integration_name="mcp_srv_mgr_http"
session_data='{"protocol":"http","tools":["list_services","get_service_status"]}'
expires_at=$(date -d '+1 hour' '+%Y-%m-%d %H:%M:%S' 2>/dev/null || date -v +1H '+%Y-%m-%d %H:%M:%S' 2>/dev/null)

insert_query="INSERT INTO unla_sessions (id, user_id, integration_name, data, expires_at) VALUES ('$session_id', '$user_id', '$integration_name', '$session_data', '$expires_at');"

if mysql_query "$insert_query" true; then
    print_test_status "Session Data Insert" "PASS"
else
    print_test_status "Session Data Insert" "FAIL"
fi

# Test 5: Query session data
echo "Testing session data retrieval..."
select_query="SELECT id, user_id, integration_name FROM unla_sessions WHERE id = '$session_id';"
if result=$(mysql_query "$select_query" true) && echo "$result" | grep -q "$session_id"; then
    print_test_status "Session Data Select" "PASS"
    echo "  Retrieved session: $session_id"
else
    print_test_status "Session Data Select" "FAIL"
fi

# Test 6: Update configuration
echo "Testing configuration update..."
config_json='{"name":"mcp_srv_mgr_gateway","version":"1.0.1","enabled":true,"last_updated":"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}'
update_query="UPDATE unla_configurations SET config = JSON_SET(config, '$.version', '1.0.1', '$.last_updated', '$(date -u +%Y-%m-%dT%H:%M:%SZ)') WHERE name = 'mcp_srv_mgr';"

if mysql_query "$update_query" true; then
    print_test_status "Configuration Update" "PASS"
else
    print_test_status "Configuration Update" "FAIL"
fi

# Test 7: Insert metrics data
echo "Testing metrics data insertion..."
current_time=$(date '+%Y-%m-%d %H:%M:%S')
metrics_query="INSERT INTO unla_metrics (integration_name, tool_name, execution_time_ms, status, timestamp) VALUES ('mcp_srv_mgr_http', 'list_services', 150, 'success', '$current_time');"

if mysql_query "$metrics_query" true; then
    print_test_status "Metrics Data Insert" "PASS"
else
    print_test_status "Metrics Data Insert" "FAIL"
fi

# Test 8: Complex JSON query
echo "Testing JSON operations..."
json_query="SELECT JSON_EXTRACT(config, '$.name') as service_name, JSON_EXTRACT(config, '$.version') as version FROM unla_configurations WHERE name = 'mcp_srv_mgr';"
if result=$(mysql_query "$json_query" true) && echo "$result" | grep -q "mcp_srv_mgr_gateway"; then
    print_test_status "JSON Operations" "PASS"
    echo "  JSON query result: $(echo "$result" | tail -1)"
else
    print_test_status "JSON Operations" "FAIL"
fi

echo ""
echo "📊 Testing Performance and Indexes"
echo "-----------------------------------"

# Test 9: Index performance
echo "Testing index performance..."
explain_query="EXPLAIN SELECT * FROM unla_sessions WHERE id = '$session_id';"
if result=$(mysql_query "$explain_query" true) && echo "$result" | grep -q "PRIMARY"; then
    print_test_status "Index Performance" "PASS"
    echo "  Primary key index is being used"
else
    print_test_status "Index Performance" "FAIL"
fi

# Test 10: Transaction test
echo "Testing transaction support..."
transaction_queries="
START TRANSACTION;
INSERT INTO unla_metrics (integration_name, tool_name, execution_time_ms, status) VALUES ('test_integration', 'test_tool', 100, 'success');
UPDATE unla_configurations SET config = JSON_SET(config, '$.test_transaction', 'true') WHERE name = 'mcp_srv_mgr';
COMMIT;
"

if mysql_query "$transaction_queries" true; then
    print_test_status "Transaction Support" "PASS"
else
    print_test_status "Transaction Support" "FAIL"
fi

echo ""
echo "🔄 Testing Redis Integration"
echo "-----------------------------"

if check_redis_connection; then
    # Test 11: Redis basic operations
    echo "Testing Redis basic operations..."
    test_key="unla:test:$(date +%s)"
    test_value="test_value_$(date +%s)"
    
    if redis_command "SET $test_key '$test_value'" > /dev/null && redis_command "GET $test_key" | grep -q "$test_value"; then
        print_test_status "Redis Basic Operations" "PASS"
        # Clean up
        redis_command "DEL $test_key" > /dev/null
    else
        print_test_status "Redis Basic Operations" "FAIL"
    fi
    
    # Test 12: Redis session caching
    echo "Testing Redis session caching..."
    cache_key="unla:session:$session_id"
    cache_data='{"user_id":"'$user_id'","integration":"'$integration_name'","cached_at":"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}'
    
    if redis_command "SETEX $cache_key 3600 '$cache_data'" > /dev/null && redis_command "GET $cache_key" | grep -q "$user_id"; then
        print_test_status "Redis Session Caching" "PASS"
        # Clean up
        redis_command "DEL $cache_key" > /dev/null
    else
        print_test_status "Redis Session Caching" "FAIL"
    fi
    
    # Test 13: Redis pub/sub
    echo "Testing Redis pub/sub..."
    channel="unla:notifications"
    message="test_notification_$(date +%s)"
    
    # This is a simplified test - in real scenarios you'd have a subscriber
    if redis_command "PUBLISH $channel '$message'" | grep -q "0"; then
        print_test_status "Redis Pub/Sub" "PASS"
        echo "  Published to channel (no subscribers currently)"
    else
        print_test_status "Redis Pub/Sub" "FAIL"
    fi
    
else
    echo "  ⚠️  Redis tests skipped (Redis not available)"
    print_test_status "Redis Tests" "SKIP"
fi

echo ""
echo "🧹 Cleanup Test Data"
echo "---------------------"

# Clean up test data
echo "Cleaning up test data..."
cleanup_queries="
DELETE FROM unla_sessions WHERE id = '$session_id';
DELETE FROM unla_metrics WHERE integration_name = 'test_integration';
UPDATE unla_configurations SET config = JSON_REMOVE(config, '$.test_transaction') WHERE name = 'mcp_srv_mgr';
"

if mysql_query "$cleanup_queries" true; then
    print_test_status "Test Data Cleanup" "PASS"
else
    print_test_status "Test Data Cleanup" "FAIL"
fi

echo ""
echo "🔧 Database Health Check"
echo "-------------------------"

# Test 14: Connection pool simulation
echo "Testing multiple connections..."
connection_test=true
for i in {1..5}; do
    if ! mysql_query "SELECT CONNECTION_ID();" true > /dev/null; then
        connection_test=false
        break
    fi
done

if [ "$connection_test" = "true" ]; then
    print_test_status "Multiple Connections" "PASS"
else
    print_test_status "Multiple Connections" "FAIL"
fi

# Test 15: Database size and status
echo "Testing database status..."
if status_result=$(mysql_query "SHOW TABLE STATUS;" true) && echo "$status_result" | grep -q "unla_"; then
    print_test_status "Database Status" "PASS"
    
    # Show table sizes
    echo "  Table information:"
    mysql_query "SELECT TABLE_NAME, TABLE_ROWS, DATA_LENGTH, INDEX_LENGTH FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = '$MYSQL_DB';" true | while IFS=$'\t' read -r table_name rows data_length index_length; do
        if [ "$table_name" != "TABLE_NAME" ]; then
            echo "    $table_name: $rows rows, $(( data_length / 1024 ))KB data, $(( index_length / 1024 ))KB index"
        fi
    done
else
    print_test_status "Database Status" "FAIL"
fi

# Final Summary
echo ""
echo "🏁 Final Database Integration Test Summary"
echo "=========================================="
echo "MySQL Tests:"
echo "   Passed: $PASSED"
echo "   Failed: $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All database tests passed!${NC}"
    echo ""
    echo "Database is ready for Unla Gateway integration:"
    echo "  ✅ Tables created and accessible"
    echo "  ✅ CRUD operations working"
    echo "  ✅ JSON operations functional"
    echo "  ✅ Indexes and performance optimized"
    echo "  ✅ Transaction support verified"
    
    if check_redis_connection; then
        echo "  ✅ Redis caching available"
    else
        echo "  ⚠️  Redis caching not available (optional)"
    fi
    
    exit 0
else
    echo -e "${RED}💥 Some database tests failed!${NC}"
    echo ""
    echo "Please fix the issues before proceeding with Unla Gateway integration."
    exit 1
fi