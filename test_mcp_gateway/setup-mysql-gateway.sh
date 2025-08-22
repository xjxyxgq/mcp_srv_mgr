#!/bin/bash

# Setup MySQL database for Unla Gateway
# This script creates the necessary tables and initial configuration

set -e

# Configuration
MYSQL_HOST="127.0.0.1"
MYSQL_PORT="3311"
MYSQL_USER="root"
MYSQL_PASSWORD="nov24feb11"
MYSQL_DATABASE="unla_gateway"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔧 Setting up MySQL database for Unla Gateway${NC}"
echo "================================================="
echo ""

# Function to execute MySQL command
mysql_exec() {
    local query="$1"
    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -e "$query" 2>/dev/null
}

# Function to check MySQL connection
check_mysql_connection() {
    echo -n "Checking MySQL connection... "
    if mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" -e "SELECT 1;" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Connected${NC}"
        return 0
    else
        echo -e "${RED}❌ Failed${NC}"
        return 1
    fi
}

# Function to check if database exists
check_database() {
    echo -n "Checking database '$MYSQL_DATABASE'... "
    if mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" -e "USE $MYSQL_DATABASE;" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Exists${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠️  Creating database${NC}"
        mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" -e "CREATE DATABASE IF NOT EXISTS $MYSQL_DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
        echo -e "${GREEN}✅ Database created${NC}"
        return 0
    fi
}

# Check prerequisites
if ! command -v mysql &> /dev/null; then
    echo -e "${RED}❌ MySQL client not found. Please install mysql-client.${NC}"
    exit 1
fi

# Check MySQL connection
if ! check_mysql_connection; then
    echo -e "${RED}❌ Cannot connect to MySQL. Please ensure MySQL is running:${NC}"
    echo "   docker compose up -d mysql"
    exit 1
fi

# Check/create database
check_database

echo ""
echo -e "${BLUE}Creating Unla Gateway tables...${NC}"

# Create mcp_configs table
echo -n "Creating mcp_configs table... "
mysql_exec "
CREATE TABLE IF NOT EXISTS mcp_configs (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    tenant VARCHAR(50) NOT NULL DEFAULT 'default',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    routers TEXT,
    servers TEXT,
    tools TEXT,
    prompts TEXT,
    mcp_servers TEXT,
    deleted_at DATETIME NULL,
    UNIQUE INDEX idx_name_tenant (tenant, name),
    INDEX idx_mcp_configs_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
"
echo -e "${GREEN}✅ Created${NC}"

# Create mcp_config_versions table
echo -n "Creating mcp_config_versions table... "
mysql_exec "
CREATE TABLE IF NOT EXISTS mcp_config_versions (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    tenant VARCHAR(50) NOT NULL,
    version INT NOT NULL,
    action_type VARCHAR(50) NOT NULL,
    created_by VARCHAR(100) NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    routers TEXT,
    servers TEXT,
    tools TEXT,
    prompts TEXT,
    mcp_servers TEXT,
    hash VARCHAR(255) NOT NULL,
    deleted_at DATETIME NULL,
    INDEX idx_name_tenant_version (name, tenant, version),
    INDEX idx_mcp_config_versions_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
"
echo -e "${GREEN}✅ Created${NC}"

# Create active_versions table
echo -n "Creating active_versions table... "
mysql_exec "
CREATE TABLE IF NOT EXISTS active_versions (
    id INT AUTO_INCREMENT PRIMARY KEY,
    tenant VARCHAR(50) NOT NULL,
    name VARCHAR(50) NOT NULL,
    version INT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL,
    UNIQUE INDEX idx_tenant_name (tenant, name),
    INDEX idx_active_versions_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
"
echo -e "${GREEN}✅ Created${NC}"

echo ""
echo -e "${BLUE}Inserting mcp_srv_mgr configuration...${NC}"

# Insert mcp_srv_mgr configuration
CONFIG_NAME="mcp_srv_mgr"
TENANT="default"

# Create JSON data for configuration
ROUTERS_JSON='[{"server": "mcp_srv_mgr", "prefix": "/mcp-service-manager", "cors": {"allowOrigins": ["*"], "allowMethods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"], "allowHeaders": ["Content-Type", "Authorization", "Mcp-Session-Id", "mcp-protocol-version"], "exposeHeaders": ["Mcp-Session-Id", "mcp-protocol-version"], "allowCredentials": true}}]'

SERVERS_JSON='[{"name": "mcp_srv_mgr", "description": "Linux Service Management System", "allowedTools": ["list_services", "get_service_status", "start_service", "stop_service", "restart_service", "enable_service", "disable_service", "get_docker_logs"]}]'

TOOLS_JSON='[
    {"name": "list_services", "description": "List all available services", "method": "GET", "endpoint": "http://localhost:8080/services"},
    {"name": "get_service_status", "description": "Get service status", "method": "GET", "endpoint": "http://localhost:8080/services/{service_name}/status"},
    {"name": "start_service", "description": "Start a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/start"},
    {"name": "stop_service", "description": "Stop a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/stop"},
    {"name": "restart_service", "description": "Restart a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/restart"},
    {"name": "enable_service", "description": "Enable a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/enable"},
    {"name": "disable_service", "description": "Disable a service", "method": "POST", "endpoint": "http://localhost:8080/services/{service_name}/disable"},
    {"name": "get_docker_logs", "description": "Get Docker logs", "method": "GET", "endpoint": "http://localhost:8080/docker/{container_name}/logs"}
]'

PROMPTS_JSON='[]'
MCP_SERVERS_JSON='[]'

echo -n "Inserting configuration data... "

# Insert main configuration
mysql_exec "
INSERT INTO mcp_configs (name, tenant, routers, servers, tools, prompts, mcp_servers)
VALUES ('$CONFIG_NAME', '$TENANT', '$ROUTERS_JSON', '$SERVERS_JSON', '$TOOLS_JSON', '$PROMPTS_JSON', '$MCP_SERVERS_JSON')
ON DUPLICATE KEY UPDATE
    updated_at = CURRENT_TIMESTAMP,
    routers = VALUES(routers),
    servers = VALUES(servers),
    tools = VALUES(tools),
    prompts = VALUES(prompts),
    mcp_servers = VALUES(mcp_servers);
"

# Insert version record
mysql_exec "
INSERT INTO mcp_config_versions (name, tenant, version, action_type, routers, servers, tools, prompts, mcp_servers, hash)
VALUES ('$CONFIG_NAME', '$TENANT', 1, 'create', '$ROUTERS_JSON', '$SERVERS_JSON', '$TOOLS_JSON', '$PROMPTS_JSON', '$MCP_SERVERS_JSON', 'mysql_setup')
ON DUPLICATE KEY UPDATE
    updated_at = CURRENT_TIMESTAMP;
"

# Insert active version
mysql_exec "
INSERT INTO active_versions (tenant, name, version)
VALUES ('$TENANT', '$CONFIG_NAME', 1)
ON DUPLICATE KEY UPDATE
    version = VALUES(version),
    updated_at = CURRENT_TIMESTAMP;
"

echo -e "${GREEN}✅ Configuration inserted${NC}"

echo ""
echo -e "${BLUE}Verifying setup...${NC}"

# Verify tables exist
echo -n "Verifying tables... "
table_count=$(mysql_exec "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = '$MYSQL_DATABASE' AND table_name IN ('mcp_configs', 'mcp_config_versions', 'active_versions');" | tail -1)
if [ "$table_count" -eq 3 ]; then
    echo -e "${GREEN}✅ All tables created${NC}"
else
    echo -e "${RED}❌ Missing tables${NC}"
    exit 1
fi

# Verify configuration
echo -n "Verifying configuration... "
config_count=$(mysql_exec "SELECT COUNT(*) FROM mcp_configs WHERE name = '$CONFIG_NAME';" | tail -1)
if [ "$config_count" -gt 0 ]; then
    echo -e "${GREEN}✅ Configuration exists${NC}"
else
    echo -e "${RED}❌ Configuration missing${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}🎉 MySQL setup completed successfully!${NC}"
echo ""
echo -e "${YELLOW}Database connection details:${NC}"
echo "  Host: $MYSQL_HOST"
echo "  Port: $MYSQL_PORT"
echo "  Database: $MYSQL_DATABASE"
echo "  User: $MYSQL_USER"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Start Unla Gateway with MySQL configuration:"
echo "   ./mcp-gateway --conf test_mcp_gateway/mcp-gateway-mysql.yaml"
echo ""
echo "2. Test the integration:"
echo "   ./test_mcp_gateway/run_all_tests.sh"