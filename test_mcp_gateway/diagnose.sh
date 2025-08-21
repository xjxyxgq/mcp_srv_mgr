#!/bin/bash

echo "🔍 Diagnosing Integration Test Issues"
echo "====================================="

cd /Users/xuguoqiang/SynologyDrive/Backup/MI_office_notebook/D/myworkspace/nucc_workspace/program/src/nucc.com/mcp_srv_mgr

echo ""
echo "1. Checking current directory and files:"
echo "Current directory: $(pwd)"
ls -la mcp-server 2>/dev/null || echo "mcp-server not found"

echo ""
echo "2. Checking Go build:"
if go version > /dev/null 2>&1; then
    echo "Go is available: $(go version)"
    if go build -o mcp-server cmd/server/main.go; then
        echo "✅ mcp-server built successfully"
        ls -la mcp-server
    else
        echo "❌ Failed to build mcp-server"
    fi
else
    echo "❌ Go not available"
fi

echo ""
echo "3. Testing basic server execution:"
if ./mcp-server -help > /dev/null 2>&1; then
    echo "✅ mcp-server help works"
else
    echo "❌ mcp-server help failed"
fi

echo ""
echo "4. Checking MySQL availability:"
if mysql --version > /dev/null 2>&1; then
    echo "MySQL client available: $(mysql --version)"
    
    # Test connection to localhost:3311
    if mysql -h 127.0.0.1 -P 3311 -u root -pnov24feb11 -e "SELECT 1 as test;" 2>/dev/null; then
        echo "✅ MySQL connection successful"
    else
        echo "❌ MySQL connection failed (127.0.0.1:3311)"
        echo "You need to start MySQL on port 3311 with user root/password nov24feb11"
        
        echo ""
        echo "To start MySQL with Docker:"
        echo "  docker run -d --name test-mysql -p 3311:3306 -e MYSQL_ROOT_PASSWORD=nov24feb11 mysql:8.0"
        
        echo ""
        echo "Or start the containers from the compose file:"
        echo "  First start Docker Desktop, then run:"
        echo "  docker compose -f test_mcp_gateway/docker-compose.yml up -d"
    fi
else
    echo "❌ MySQL client not available"
fi

echo ""
echo "5. Checking Docker availability:"
if docker --version > /dev/null 2>&1; then
    echo "Docker available: $(docker --version)"
    if docker ps > /dev/null 2>&1; then
        echo "✅ Docker daemon is running"
        echo "Running containers:"
        docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    else
        echo "❌ Docker daemon not running"
        echo "Please start Docker Desktop"
    fi
else
    echo "❌ Docker not available"
fi

echo ""
echo "6. Quick HTTP test:"
PORT=8080
echo "Testing if port $PORT is available..."
if lsof -i :$PORT > /dev/null 2>&1; then
    echo "⚠️  Port $PORT is already in use"
    lsof -i :$PORT
else
    echo "✅ Port $PORT is available"
fi