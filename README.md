# Linux Service Manager

A comprehensive service management server for Linux systems, supporting multiple service management methods including systemd, System V init, and Docker containers. 

**Now supports MCP (Model Context Protocol) for integration with AI models like Claude!**

## Features

- **Multi-platform support**: systemd, System V init, Docker
- **Dual interface**: Both MCP protocol and RESTful HTTP API
- **MCP integration**: Native support for AI models via Model Context Protocol
- **Auto-detection**: Automatically detects available service managers
- **Docker integration**: Manage Docker containers as services
- **AI-friendly tools**: Pre-defined tools and prompts for AI model interaction
- **Comprehensive logging**: Configurable logging with multiple formats
- **Configuration management**: YAML-based configuration with environment variable overrides

## Supported Service Types

### 1. systemd Services
- Start, stop, restart services
- Enable/disable services for boot
- Get detailed service status and logs
- List all systemd services

### 2. System V init Services
- Control services via `/etc/init.d` scripts
- Support for `chkconfig` and `update-rc.d`
- Extract service information from LSB headers
- Compatible with legacy Linux distributions

### 3. Docker Containers
- Start, stop, restart containers
- List all containers with status
- Get container logs and statistics
- Create and remove containers
- Manage container restart policies

## Installation

```bash
# Clone the repository
git clone <repository-url>
cd mcp_srv_mgr

# Build the application
make build
# or manually:
go build -o mcp-server ./cmd/server

# For MCP mode (AI model integration)
./mcp-server -mcp

# For HTTP API mode
./mcp-server -config config.yaml

# Generate default configuration (HTTP mode)
./mcp-server -generate-config
```

## MCP (Model Context Protocol) Usage

This server can be used with AI models like Claude through the MCP protocol.

### Claude Desktop Integration

1. **Build the MCP server:**
```bash
go build -o mcp-server
```

2. **Add to Claude Desktop configuration:**

Edit your Claude Desktop configuration file (typically at `~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "linux-service-manager": {
      "command": "/full/path/to/your/mcp-server",
      "args": ["-mcp", "-log-level", "error"]
    }
  }
}
```

3. **Restart Claude Desktop** - The service manager tools will be available in your Claude conversations.

### Available MCP Tools

When connected via MCP, the following tools are available to AI models:

- **`list_services`** - List all available services (with optional type filtering)
- **`get_service_status`** - Get detailed status of a specific service
- **`start_service`** - Start a service
- **`stop_service`** - Stop a service  
- **`restart_service`** - Restart a service
- **`enable_service`** - Enable a service for automatic startup
- **`disable_service`** - Disable a service from automatic startup
- **`get_docker_logs`** - Get logs from a Docker container

### Available MCP Prompts

- **`service_management_help`** - Get comprehensive help for managing Linux services
- **`service_troubleshooting`** - Get troubleshooting guidance for service issues

### Example MCP Usage

Once configured with Claude Desktop, you can ask questions like:

- "Show me all running systemd services"
- "What's the status of the nginx service?"
- "Start the docker container named web-server"
- "Help me troubleshoot why my mysql service won't start"
- "Get the last 50 lines of logs from my app container"

The AI model will use the appropriate tools to manage your services and provide helpful responses.

## Configuration

### Configuration File (config.yaml)

```yaml
server:
  host: "127.0.0.1"
  port: 8080

log:
  level: "info"      # debug, info, warn, error
  format: "json"     # json, text
  output: "stdout"   # stdout, stderr
```

### Environment Variables

- `MCP_HOST`: Server host (default: 127.0.0.1)
- `MCP_PORT`: Server port (default: 8080)
- `MCP_LOG_LEVEL`: Log level (default: info)
- `MCP_LOG_FORMAT`: Log format (default: json)

## API Endpoints

### Service Management

#### List All Services
```http
GET /services
GET /services?type=systemd
GET /services?type=sysv
GET /services?type=docker
```

#### Get Service Status
```http
GET /services/{name}/status
GET /services/{name}/status?type=systemd
```

#### Service Operations
```http
POST /services/{name}/start
POST /services/{name}/stop
POST /services/{name}/restart
POST /services/{name}/enable
POST /services/{name}/disable
```

#### Generic Service Action
```http
POST /services/action
Content-Type: application/json

{
  "name": "nginx",
  "type": "systemd",
  "action": "start"
}
```

### Docker-Specific Endpoints

#### Get Container Logs
```http
GET /docker/{name}/logs?lines=100
```

#### Get Container Statistics
```http
GET /docker/{name}/stats
```

#### Remove Container
```http
DELETE /docker/{name}/remove?force=true
```

#### Create Container
```http
POST /docker/create
Content-Type: application/json

{
  "image_name": "nginx:latest",
  "container_name": "my-nginx",
  "options": ["-p", "80:80"]
}
```

### System Endpoints

#### Health Check
```http
GET /health
```

#### Server Information
```http
GET /info
```

## Usage Examples

### Using curl

```bash
# List all services
curl http://localhost:8080/services

# Get nginx service status
curl http://localhost:8080/services/nginx/status

# Start nginx service
curl -X POST http://localhost:8080/services/nginx/start

# Stop docker container
curl -X POST http://localhost:8080/services/my-container/stop?type=docker

# Get docker container logs
curl http://localhost:8080/docker/my-container/logs?lines=50

# Health check
curl http://localhost:8080/health
```

### Response Format

#### Service List Response
```json
{
  "success": true,
  "message": "Services listed successfully",
  "services": [
    {
      "name": "nginx",
      "type": "systemd",
      "status": "active",
      "description": "The nginx HTTP and reverse proxy server",
      "pid": 1234,
      "uptime": "2h30m15s",
      "last_changed": "2023-01-01T10:00:00Z"
    }
  ]
}
```

#### Service Status Response
```json
{
  "success": true,
  "message": "Service status retrieved successfully",
  "service": {
    "name": "nginx",
    "type": "systemd",
    "status": "active",
    "description": "The nginx HTTP and reverse proxy server",
    "pid": 1234,
    "uptime": "2h30m15s",
    "last_changed": "2023-01-01T10:00:00Z"
  }
}
```

## Command Line Options

```bash
# Show version
./mcp-server -version

# Run in MCP mode (for AI model integration)
./mcp-server -mcp

# Run in MCP mode with debug logging
./mcp-server -mcp -log-level debug

# Run in HTTP API mode (default)
./mcp-server -config /path/to/config.yaml

# Generate default HTTP configuration
./mcp-server -generate-config

# Show help
./mcp-server -help
```

## Requirements

- Go 1.21 or later
- Linux operating system
- Root privileges for some service operations
- systemd (optional, for systemd services)
- Docker (optional, for container management)
- Claude Desktop (for MCP integration with AI models)

## Security Considerations

- The server requires appropriate permissions to manage services
- Consider running with restricted privileges where possible
- Use firewall rules to restrict access to the API endpoints
- Enable authentication/authorization for production use

## Error Handling

The server provides comprehensive error handling with appropriate HTTP status codes:

- `200 OK`: Successful operation
- `400 Bad Request`: Invalid request parameters
- `404 Not Found`: Service not found
- `500 Internal Server Error`: Operation failed
- `503 Service Unavailable`: Service manager not available

## Logging

All operations are logged with configurable levels and formats:

- **Levels**: debug, info, warn, error
- **Formats**: json, text
- **Output**: stdout, stderr

## Project Structure

The project follows a standard Go module layout for better organization and maintainability:

```
.
├── cmd/
│   └── server/          # Main application entry point
│       └── main.go
├── internal/            # Private application code
│   ├── config/          # Configuration management
│   ├── managers/        # Service manager implementations
│   │   ├── systemd.go   # systemd service manager
│   │   ├── sysv.go      # System V init manager
│   │   └── docker.go    # Docker container manager
│   ├── mcp/             # MCP protocol server
│   │   └── server.go
│   └── server/          # HTTP REST API server
│       └── http.go
├── pkg/                 # Public libraries
│   ├── types/           # Shared type definitions
│   │   ├── service.go   # Service-related types
│   │   └── mcp.go       # MCP protocol types
│   └── utils/           # Utility functions
│       └── utils.go
├── test/                # Integration tests
│   └── integration_test.go
├── go.mod               # Go module definition
├── go.sum               # Go module checksums
├── Makefile             # Build automation
├── Dockerfile           # Container build instructions
├── docker-compose.yaml  # Multi-service container setup
└── README.md            # This file
```

### Key Components

- **cmd/server**: Main application entry point with CLI argument parsing
- **internal/config**: Configuration loading and environment variable handling
- **internal/managers**: Service manager implementations for different systems
- **internal/mcp**: MCP protocol server for AI model integration
- **internal/server**: HTTP REST API server implementation
- **pkg/types**: Shared data structures and type definitions
- **pkg/utils**: Common utility functions
- **test/**: Integration tests that require actual system resources

## Development

### Building

```bash
make build           # Build the application
make build-all       # Build for all supported platforms
```

### Testing

```bash
make test            # Run unit tests
make test-all        # Run all tests including integration
make test-coverage   # Generate coverage report
make benchmark       # Run performance benchmarks
```

### Docker

```bash
make docker-build    # Build Docker image
make docker-run      # Run in Docker container
make docker-test     # Run tests in Docker
```

## License

This project is licensed under the MIT License.