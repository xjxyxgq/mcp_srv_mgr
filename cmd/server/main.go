package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"nucc.com/mcp_srv_mgr/internal/config"
	"nucc.com/mcp_srv_mgr/internal/mcp"
	"nucc.com/mcp_srv_mgr/internal/server"
)

func main() {
	var (
		configPath    = flag.String("config", "config.yaml", "Path to configuration file")
		mode          = flag.String("mode", "mcp", "Server mode: mcp, http, mcp-http, or mcp-streamable")
		httpMode      = flag.Bool("http", false, "Start HTTP server instead of MCP server")
		mcpHTTP       = flag.Bool("mcp-http", false, "Start MCP over HTTP (SSE) server")
		mcpStreamable = flag.Bool("mcp-streamable", false, "Start MCP Streamable HTTP server")
		help          = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// HTTP mode takes precedence
	if *httpMode {
		*mode = "http"
	}
	
	// MCP HTTP mode takes precedence
	if *mcpHTTP {
		*mode = "mcp-http"
	}
	
	// MCP Streamable mode takes precedence
	if *mcpStreamable {
		*mode = "mcp-streamable"
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Setup logger
	logger := logrus.New()
	switch cfg.Log.Level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	logger.Infof("Starting MCP Service Manager in %s mode", *mode)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	switch *mode {
	case "http":
		startHTTPServer(cfg, logger, sigChan)
	case "mcp":
		startMCPServer(logger, sigChan)
	case "mcp-http":
		startMCPHTTPServer(cfg, logger, sigChan)
	case "mcp-streamable":
		startMCPStreamableServer(cfg, logger, sigChan)
	default:
		logger.Fatalf("Unknown mode: %s. Use 'mcp', 'http', 'mcp-http', or 'mcp-streamable'", *mode)
	}
}

func startHTTPServer(cfg *config.Config, logger *logrus.Logger, sigChan chan os.Signal) {
	httpServer := server.NewHTTPServer(cfg, logger)

	go func() {
		logger.Info("Starting HTTP server...")
		if err := httpServer.Start(); err != nil {
			logger.Fatalf("HTTP server failed: %v", err)
		}
	}()

	<-sigChan
	logger.Info("Shutting down HTTP server...")
}

func startMCPServer(logger *logrus.Logger, sigChan chan os.Signal) {
	mcpServer := mcp.NewServer(logger)

	go func() {
		logger.Info("Starting MCP server...")
		mcpServer.Start()
	}()

	<-sigChan
	logger.Info("Shutting down MCP server...")
}

func startMCPHTTPServer(cfg *config.Config, logger *logrus.Logger, sigChan chan os.Signal) {
	mcpHTTPServer := server.NewMCPHTTPServer(cfg, logger)

	go func() {
		logger.Info("Starting MCP HTTP server...")
		if err := mcpHTTPServer.Start(); err != nil {
			logger.Fatalf("MCP HTTP server failed: %v", err)
		}
	}()

	<-sigChan
	logger.Info("Shutting down MCP HTTP server...")
}

func startMCPStreamableServer(cfg *config.Config, logger *logrus.Logger, sigChan chan os.Signal) {
	mcpStreamableServer := server.NewMCPStreamableServer(cfg, logger)

	go func() {
		logger.Info("Starting MCP Streamable server...")
		if err := mcpStreamableServer.Start(); err != nil {
			logger.Fatalf("MCP Streamable server failed: %v", err)
		}
	}()

	<-sigChan
	logger.Info("Shutting down MCP Streamable server...")
}

func showHelp() {
	fmt.Printf(`MCP Service Manager

A server for managing Linux services through MCP (Model Context Protocol) or HTTP API.

Usage:
  %s [options]

Options:
  -config string
        Path to configuration file (default "config.yaml")
  -mode string
        Server mode: mcp, http, mcp-http, or mcp-streamable (default "mcp")
  -http
        Start HTTP server instead of MCP server (same as -mode=http)
  -mcp-http
        Start MCP over HTTP (SSE) server (same as -mode=mcp-http)
  -mcp-streamable
        Start MCP Streamable HTTP server (same as -mode=mcp-streamable)
  -help
        Show this help message

Examples:
  # Start MCP server (default mode)
  %s

  # Start HTTP REST API server
  %s -http
  %s -mode=http

  # Start MCP over HTTP (SSE) server
  %s -mcp-http
  %s -mode=mcp-http

  # Start MCP Streamable HTTP server
  %s -mcp-streamable
  %s -mode=mcp-streamable

  # Use custom config file
  %s -config=/path/to/config.yaml

Supported service types:
  - systemd (modern Linux distributions)
  - System V init (traditional Linux distributions)
  - Docker containers

The server will automatically detect available service managers on your system.
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}