#!/bin/bash

# Insert MCP configuration into MySQL database
# This script inserts the mcp_srv_mgr configuration into MySQL

set -e

# Configuration
MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3311"
MYSQL_USER="root"
MYSQL_PASSWORD="nov24feb11"
MYSQL_DATABASE="unla_gateway"
CONFIG_NAME="mcp_srv_mgr"
TENANT="default"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}🔧 Inserting mcp_srv_mgr configuration into MySQL${NC}"
echo "=================================================="

# Create configuration data
cat > /tmp/mcp_config.sql << 'EOF'
-- Insert mcp_srv_mgr configuration
INSERT INTO mcp_configs (name, tenant, routers, servers, tools, prompts, mcp_servers, created_at, updated_at)
VALUES (
    'mcp_srv_mgr', 
    'default',
    '[{"server": "mcp_srv_mgr", "prefix": "/mcp-service-manager", "cors": {"allowOrigins": ["*"], "allowMethods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"], "allowHeaders": ["Content-Type", "Authorization", "Mcp-Session-Id", "mcp-protocol-version"], "exposeHeaders": ["Mcp-Session-Id", "mcp-protocol-version"], "allowCredentials": true}}]',
    '[{"name": "mcp_srv_mgr", "description": "Linux Service Management System", "allowedTools": ["list_services", "get_service_status", "start_service", "stop_service", "restart_service", "enable_service", "disable_service", "get_docker_logs"]}]',
    '[{"name": "list_services", "description": "List all available services", "method": "GET", "endpoint": "http://localhost:8080/services"}, {"name": "get_service_status", "description": "Get service status", "method": "GET", "endpoint": "http://localhost:8080/services/{service_name}/status"}, {"name": "start_service", "description": "Start a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/start"}, {"name": "stop_service", "description": "Stop a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/stop"}, {"name": "restart_service", "description": "Restart a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/restart"}, {"name": "enable_service", "description": "Enable a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/enable"}, {"name": "disable_service", "description": "Disable a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/disable"}, {"name": "get_docker_logs", "description": "Get Docker logs", "method": "GET", "endpoint": "http://localhost:8080/docker/{container_name}/logs"}]',
    '[]',
    '[]',
    NOW(),
    NOW()
)
ON DUPLICATE KEY UPDATE
    updated_at = NOW(),
    routers = VALUES(routers),
    servers = VALUES(servers),
    tools = VALUES(tools),
    prompts = VALUES(prompts),
    mcp_servers = VALUES(mcp_servers);

-- Insert version record
INSERT INTO mcp_config_versions (name, tenant, version, action_type, routers, servers, tools, prompts, mcp_servers, hash, created_at)
VALUES (
    'mcp_srv_mgr',
    'default', 
    1,
    'create',
    '[{"server": "mcp_srv_mgr", "prefix": "/mcp-service-manager", "cors": {"allowOrigins": ["*"], "allowMethods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"], "allowHeaders": ["Content-Type", "Authorization", "Mcp-Session-Id", "mcp-protocol-version"], "exposeHeaders": ["Mcp-Session-Id", "mcp-protocol-version"], "allowCredentials": true}}]',
    '[{"name": "mcp_srv_mgr", "description": "Linux Service Management System", "allowedTools": ["list_services", "get_service_status", "start_service", "stop_service", "restart_service", "enable_service", "disable_service", "get_docker_logs"]}]',
    '[{"name": "list_services", "description": "List all available services", "method": "GET", "endpoint": "http://localhost:8080/services"}, {"name": "get_service_status", "description": "Get service status", "method": "GET", "endpoint": "http://localhost:8080/services/{service_name}/status"}, {"name": "start_service", "description": "Start a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/start"}, {"name": "stop_service", "description": "Stop a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/stop"}, {"name": "restart_service", "description": "Restart a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/restart"}, {"name": "enable_service", "description": "Enable a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/enable"}, {"name": "disable_service", "description": "Disable a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/disable"}, {"name": "get_docker_logs", "description": "Get Docker logs", "method": "GET", "endpoint": "http://localhost:8080/docker/{container_name}/logs"}]',
    '[]',
    '[]',
    'mysql_import',
    NOW()
);

-- Insert active version
INSERT INTO active_versions (tenant, name, version, updated_at)
VALUES ('default', 'mcp_srv_mgr', 1, NOW())
ON DUPLICATE KEY UPDATE
    version = VALUES(version),
    updated_at = NOW();
EOF

# Execute the SQL script
echo -n "Inserting configuration... "
mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" < /tmp/mcp_config.sql 2>/dev/null

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Success${NC}"
else
    echo "❌ Failed"
    exit 1
fi

# Verify the insertion
echo -n "Verifying configuration... "
config_count=$(mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -e "SELECT COUNT(*) FROM mcp_configs WHERE name='mcp_srv_mgr';" -s -N 2>/dev/null)

if [ "$config_count" -gt 0 ]; then
    echo -e "${GREEN}✅ Configuration found${NC}"
else
    echo "❌ Configuration not found"
    exit 1
fi

# Clean up temp file
rm -f /tmp/mcp_config.sql

echo ""
echo -e "${GREEN}🎉 Configuration successfully inserted into MySQL!${NC}"
echo ""
echo "You can now start Unla Gateway with MySQL:"
echo "  ./mcp-gateway --conf test_mcp_gateway/mcp-gateway-mysql.yaml"