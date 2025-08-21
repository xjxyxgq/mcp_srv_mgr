# Unla Gateway Integration Tests for mcp_srv_mgr

This directory contains comprehensive integration tests for connecting the `mcp_srv_mgr` service with the Unla MCP Gateway. The tests verify that all supported protocols (HTTP API, MCP stdio, MCP over HTTP SSE, MCP Streamable HTTP) work correctly through the gateway.

## Overview

The `mcp_srv_mgr` is a Linux service management system that provides multiple protocol interfaces:
- **HTTP REST API**: Traditional REST endpoints for service management
- **MCP stdio**: Native MCP protocol via stdin/stdout for AI model integration
- **MCP over HTTP (SSE)**: MCP protocol over HTTP with Server-Sent Events
- **MCP Streamable HTTP**: MCP protocol with bidirectional streaming support

The Unla Gateway acts as a proxy that can integrate these services and expose them through a unified MCP interface to AI models.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│                 │    │                 │    │                 │
│   AI Models     │◄───┤  Unla Gateway   │◄───┤  mcp_srv_mgr    │
│  (Claude, etc)  │    │                 │    │                 │
│                 │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │                        │
                              │                        ├─ HTTP API (8080)
                              │                        ├─ MCP stdio
                              │                        ├─ MCP HTTP SSE (8082)
                              │                        └─ MCP Streamable (8083)
                              │
                        ┌─────────────────┐
                        │                 │
                        │  MySQL + Redis  │
                        │                 │
                        └─────────────────┘
```

## File Structure

```
test_mcp_gateway/
├── README.md                          # This documentation
├── unla-config.yaml                   # Unla Gateway configuration
├── config.yaml                        # mcp_srv_mgr HTTP mode config
├── config-mcp-http.yaml              # mcp_srv_mgr MCP HTTP mode config
├── config-mcp-streamable.yaml        # mcp_srv_mgr MCP Streamable mode config
├── docker-compose.yml                # MySQL and Redis containers
├── init.sql                          # Database initialization
├── test_http_api.sh                  # HTTP API integration tests
├── test_mcp_stdio.sh                 # MCP stdio protocol tests
├── test_mcp_http_sse.sh              # MCP over HTTP SSE tests
├── test_mcp_streamable.sh            # MCP Streamable HTTP tests
├── test_mysql_integration.sh         # Database integration tests
├── run_all_tests.sh                  # Master test runner
└── test_readme.md                    # Original requirements
```

## Prerequisites

### Required Software
- **Go** (for building mcp-server)
- **MySQL client** (for database tests)
- **Redis CLI** (for cache tests) 
- **curl** (for HTTP tests)
- **jq** (for JSON processing, optional)
- **Docker & Docker Compose** (for database containers)

### Required Services
- **Unla Gateway binary**: `mcp-gateway` should be available in the current directory
- **MySQL Database**: On localhost:3311 with credentials root/nov24feb11
- **Redis Cache**: On localhost:6379 (optional, for caching tests)

## Quick Start

### 1. Build mcp_srv_mgr
```bash
# From the main project directory
go build -o mcp-server cmd/server/main.go
```

### 2. Start Database Services
```bash
# Start MySQL and Redis containers
docker-compose -f test_mcp_gateway/docker-compose.yml up -d
```

### 3. Run All Tests
```bash
# Run the complete test suite
./test_mcp_gateway/run_all_tests.sh
```

### 4. Start Unla Gateway (after tests pass)
```bash
# Start the gateway with the generated configuration
./mcp-gateway --config test_mcp_gateway/unla-config.yaml
```

## Individual Test Scripts

### HTTP API Integration Tests (`test_http_api.sh`)
Tests the HTTP REST API endpoints both directly and through the Unla Gateway:
- Health checks
- Service listing
- Service status queries
- Service operations (start/stop/restart)
- Docker operations
- Gateway proxy functionality

**Usage:**
```bash
./test_mcp_gateway/test_http_api.sh
```

### MCP stdio Protocol Tests (`test_mcp_stdio.sh`)
Tests the native MCP protocol via stdin/stdout:
- MCP session initialization
- Tool listing and execution
- Prompt management
- Gateway stdio proxy

**Usage:**
```bash
./test_mcp_gateway/test_mcp_stdio.sh
```

### MCP HTTP SSE Tests (`test_mcp_http_sse.sh`)
Tests MCP over HTTP with Server-Sent Events:
- HTTP-based MCP communication
- SSE streaming
- Tool execution via HTTP
- Gateway HTTP proxy

**Usage:**
```bash
./test_mcp_gateway/test_mcp_http_sse.sh
```

### MCP Streamable HTTP Tests (`test_mcp_streamable.sh`)
Tests the bidirectional streaming MCP protocol:
- WebSocket-like communication
- Streaming tool execution
- Long-running operations
- Gateway streaming proxy

**Usage:**
```bash
./test_mcp_gateway/test_mcp_streamable.sh
```

### MySQL Integration Tests (`test_mysql_integration.sh`)
Tests database connectivity and operations:
- MySQL connection and table creation
- CRUD operations
- JSON data handling
- Redis caching operations
- Transaction support

**Usage:**
```bash
./test_mcp_gateway/test_mysql_integration.sh
```

## Configuration Files

### `unla-config.yaml`
Main configuration file for the Unla Gateway that defines:
- **Database settings**: MySQL connection for session management
- **Redis settings**: Cache configuration
- **Server settings**: Gateway server configuration
- **Integration definitions**: How to connect to mcp_srv_mgr services
- **Tool mappings**: REST API to MCP tool conversions

Key sections:
- `database`: MySQL configuration (host: 127.0.0.1:3311)
- `redis`: Redis configuration (host: 127.0.0.1:6379)
- `integrations`: Service integration definitions
  - `mcp_srv_mgr_http`: HTTP API integration
  - `mcp_srv_mgr_stdio`: MCP stdio integration  
  - `mcp_srv_mgr_http_sse`: MCP over HTTP integration
  - `mcp_srv_mgr_streamable`: MCP Streamable integration

### Service Configuration Files
- `config.yaml`: HTTP mode configuration (port 8080)
- `config-mcp-http.yaml`: MCP HTTP mode configuration (port 8082)
- `config-mcp-streamable.yaml`: MCP Streamable mode configuration (port 8083)

## Database Schema

The MySQL database (`unla_gateway`) contains these tables:

### `unla_sessions`
Stores active sessions and their state:
```sql
id VARCHAR(255) PRIMARY KEY           -- Session identifier
user_id VARCHAR(255)                  -- User identifier  
integration_name VARCHAR(255)         -- Which integration is being used
data TEXT                            -- Session data (JSON)
created_at TIMESTAMP                 -- Creation time
updated_at TIMESTAMP                 -- Last update time
expires_at TIMESTAMP                 -- Expiration time
```

### `unla_configurations`
Stores gateway and integration configurations:
```sql
id INT AUTO_INCREMENT PRIMARY KEY    -- Configuration ID
name VARCHAR(255) UNIQUE             -- Configuration name
config JSON                          -- Configuration data
version INT                          -- Configuration version
created_at TIMESTAMP                 -- Creation time
updated_at TIMESTAMP                 -- Last update time
```

### `unla_metrics`
Stores operational metrics:
```sql
id INT AUTO_INCREMENT PRIMARY KEY    -- Metric ID
integration_name VARCHAR(255)        -- Integration name
tool_name VARCHAR(255)              -- Tool name
execution_time_ms INT               -- Execution time
status VARCHAR(50)                  -- Status (success/error)
error_message TEXT                  -- Error details (if any)
timestamp TIMESTAMP                 -- Metric timestamp
```

## Supported Tools

The gateway exposes these MCP tools from mcp_srv_mgr:

### Service Management Tools
- `list_services`: List all available services
- `get_service_status`: Get status of a specific service
- `start_service`: Start a service
- `stop_service`: Stop a service  
- `restart_service`: Restart a service
- `enable_service`: Enable service at boot
- `disable_service`: Disable service at boot

### Docker-Specific Tools
- `get_docker_logs`: Get container logs

### Service Types Supported
- **systemd**: Modern Linux service management
- **sysv**: Traditional System V init services
- **docker**: Docker container management

## Testing Strategy

### Test Phases
1. **Prerequisites Check**: Verify required tools and dependencies
2. **Environment Setup**: Start database containers and prepare environment
3. **Database Integration**: Test MySQL and Redis connectivity and operations
4. **Direct Protocol Tests**: Test each protocol directly against mcp_srv_mgr
5. **Gateway Integration Tests**: Test protocols through Unla Gateway proxy
6. **Cleanup**: Stop services and clean up test data

### Test Coverage
- ✅ **Connectivity Tests**: Verify all services can be reached
- ✅ **Protocol Tests**: Test each MCP protocol variant
- ✅ **Tool Execution**: Verify all tools work correctly
- ✅ **Error Handling**: Test error conditions and recovery
- ✅ **Performance**: Basic performance and timeout testing
- ✅ **Data Persistence**: Database operations and session management
- ✅ **Gateway Proxy**: End-to-end gateway functionality

### Test Results
Each test script provides:
- **Pass/Fail Status**: Clear indication of test results
- **Detailed Output**: Specific error messages and debugging info
- **Performance Metrics**: Execution times and success rates
- **Cleanup**: Automatic cleanup of test data

## Troubleshooting

### Common Issues

**1. MySQL Connection Failed**
```bash
# Check if MySQL is running
docker-compose -f test_mcp_gateway/docker-compose.yml ps mysql

# Check MySQL logs
docker-compose -f test_mcp_gateway/docker-compose.yml logs mysql

# Restart MySQL
docker-compose -f test_mcp_gateway/docker-compose.yml restart mysql
```

**2. mcp-server Build Failed**
```bash
# Ensure Go modules are up to date
go mod tidy

# Build with verbose output
go build -v -o mcp-server cmd/server/main.go
```

**3. Port Conflicts**
```bash
# Check what's using the ports
lsof -i :8080  # HTTP API
lsof -i :8081  # Unla Gateway
lsof -i :8082  # MCP HTTP
lsof -i :8083  # MCP Streamable
lsof -i :3311  # MySQL
lsof -i :6379  # Redis
```

**4. Gateway Connection Failed**
```bash
# Check if Unla Gateway is running
curl -f http://127.0.0.1:8081/health

# Check gateway logs
./mcp-gateway --config test_mcp_gateway/unla-config.yaml --log-level debug
```

### Debug Mode
Run individual tests with more verbose output:
```bash
# Enable debug output in scripts
export DEBUG=1
./test_mcp_gateway/test_http_api.sh

# Run with bash debugging
bash -x ./test_mcp_gateway/test_http_api.sh
```

## Integration with AI Models

Once all tests pass and the Unla Gateway is running, AI models can connect to:
- **Gateway URL**: `http://127.0.0.1:8081`
- **Protocol**: MCP over HTTP
- **Available Tools**: All mcp_srv_mgr service management tools

Example MCP client connection:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list",
  "params": {}
}
```

The gateway will proxy this to the appropriate mcp_srv_mgr instance and return the unified response.

## Security Considerations

### Current Configuration (Testing)
- **Authentication**: Disabled for testing
- **Network**: Localhost only (127.0.0.1)
- **Database**: Default passwords (change in production)

### Production Recommendations
- **Enable Authentication**: Set `security.enable_auth: true`
- **Use HTTPS**: Configure TLS certificates
- **Secure Database**: Use strong passwords and connection encryption
- **Network Security**: Use firewall rules and VPNs
- **Monitoring**: Enable metrics and logging

## Performance Considerations

### Resource Usage
- **Memory**: ~50MB per mcp_srv_mgr instance
- **CPU**: Minimal when idle, scales with request volume
- **Database**: Optimized for concurrent sessions
- **Network**: HTTP keep-alive enabled

### Scaling
- **Horizontal**: Multiple mcp_srv_mgr instances behind gateway
- **Database**: MySQL with read replicas
- **Caching**: Redis for session and response caching
- **Load Balancing**: Multiple Unla Gateway instances

## Contributing

To add new tests or modify existing ones:

1. **Follow the existing test structure**
2. **Use the standard status reporting functions**
3. **Include both direct and gateway tests**
4. **Add proper cleanup procedures**
5. **Update this documentation**

### Test Template
```bash
#!/bin/bash
# Test description here

# Standard setup
print_test_status() { ... }
make_request() { ... }

# Test implementation
echo "Testing feature X..."
if test_condition; then
    print_test_status "Feature X" "PASS"
else
    print_test_status "Feature X" "FAIL"
fi

# Cleanup
cleanup() { ... }
trap cleanup EXIT
```

---

## Summary

This integration test suite provides comprehensive verification that mcp_srv_mgr can be successfully integrated with the Unla Gateway across all supported protocols. The tests ensure:

- ✅ **Protocol Compatibility**: All MCP protocol variants work correctly
- ✅ **Tool Functionality**: Service management tools operate as expected  
- ✅ **Database Integration**: Persistent storage and caching work properly
- ✅ **Gateway Proxy**: End-to-end integration through Unla Gateway
- ✅ **Error Handling**: Graceful handling of error conditions
- ✅ **Performance**: Acceptable response times and resource usage

Run `./test_mcp_gateway/run_all_tests.sh` to execute the complete test suite and verify your integration is ready for production use.