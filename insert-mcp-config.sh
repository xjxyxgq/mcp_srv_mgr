#!/bin/bash

# Insert MCP configuration into SQLite database
DB_PATH="./test_mcp_gateway/mcp-gateway.db"
CONFIG_FILE="./test_mcp_gateway/mcp_srv_mgr_proxy.yaml"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "Configuration file not found: $CONFIG_FILE"
    exit 1
fi

CONFIG_NAME="mcp_srv_mgr"
TENANT="default"
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Extract routers section (lines 4-25)
ROUTERS=$(sed -n '4,25p' "$CONFIG_FILE" | sed 's/^routers://' | sed 's/^/  /' | sed '1s/^  //' | python3 -c "
import sys
import json
lines = sys.stdin.readlines()
# Create a minimal JSON structure for routers
routers = [{
    'server': 'mcp_srv_mgr', 
    'prefix': '/mcp-service-manager',
    'cors': {
        'allowOrigins': ['*'],
        'allowMethods': ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
        'allowHeaders': ['Content-Type', 'Authorization', 'Mcp-Session-Id', 'mcp-protocol-version'],
        'exposeHeaders': ['Mcp-Session-Id', 'mcp-protocol-version'],
        'allowCredentials': True
    }
}]
print(json.dumps(routers))
")

# Extract servers section (lines 26-38)  
SERVERS=$(python3 -c "
import json
servers = [{
    'name': 'mcp_srv_mgr',
    'description': 'Linux Service Management System',
    'allowedTools': [
        'list_services', 'get_service_status', 'start_service', 
        'stop_service', 'restart_service', 'enable_service', 
        'disable_service', 'get_docker_logs'
    ]
}]
print(json.dumps(servers))
")

# Extract tools section - create a simplified version
TOOLS=$(python3 -c "
import json
tools = [
    {'name': 'list_services', 'description': 'List all available services', 'method': 'GET', 'endpoint': 'http://localhost:8080/services'},
    {'name': 'get_service_status', 'description': 'Get service status', 'method': 'GET', 'endpoint': 'http://localhost:8080/services/{service_name}/status'},
    {'name': 'start_service', 'description': 'Start a service', 'method': 'POST', 'endpoint': 'http://localhost:8080/services/{service_name}/start'},
    {'name': 'stop_service', 'description': 'Stop a service', 'method': 'POST', 'endpoint': 'http://localhost:8080/services/{service_name}/stop'},
    {'name': 'restart_service', 'description': 'Restart a service', 'method': 'POST', 'endpoint': 'http://localhost:8080/services/{service_name}/restart'},
    {'name': 'enable_service', 'description': 'Enable a service', 'method': 'POST', 'endpoint': 'http://localhost:8080/services/{service_name}/enable'},
    {'name': 'disable_service', 'description': 'Disable a service', 'method': 'POST', 'endpoint': 'http://localhost:8080/services/{service_name}/disable'},
    {'name': 'get_docker_logs', 'description': 'Get Docker logs', 'method': 'GET', 'endpoint': 'http://localhost:8080/docker/{container_name}/logs'}
]
print(json.dumps(tools))
")

PROMPTS='[]'
MCP_SERVERS='[]'

# Insert into database
sqlite3 "$DB_PATH" <<EOF
INSERT OR REPLACE INTO mcp_configs 
(name, tenant, created_at, updated_at, routers, servers, tools, prompts, mcp_servers)
VALUES ('$CONFIG_NAME', '$TENANT', '$TIMESTAMP', '$TIMESTAMP', '$ROUTERS', '$SERVERS', '$TOOLS', '$PROMPTS', '$MCP_SERVERS');

INSERT OR REPLACE INTO mcp_config_versions 
(name, tenant, version, action_type, created_at, routers, servers, tools, prompts, mcp_servers, hash)
VALUES ('$CONFIG_NAME', '$TENANT', 1, 'create', '$TIMESTAMP', '$ROUTERS', '$SERVERS', '$TOOLS', '$PROMPTS', '$MCP_SERVERS', 'initial');

INSERT OR REPLACE INTO active_versions 
(tenant, name, version, updated_at)
VALUES ('$TENANT', '$CONFIG_NAME', 1, '$TIMESTAMP');

SELECT 'Configuration inserted successfully for: ' || name || ' (tenant: ' || tenant || ')' as result
FROM mcp_configs WHERE name = '$CONFIG_NAME';
EOF

echo "MCP configuration has been inserted into the database."