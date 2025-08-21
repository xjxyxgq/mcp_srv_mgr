-- Initialize Unla Gateway database
CREATE DATABASE IF NOT EXISTS unla_gateway;
USE unla_gateway;

-- Create tables for Unla gateway session management
CREATE TABLE IF NOT EXISTS unla_sessions (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255),
    integration_name VARCHAR(255),
    data TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    expires_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS unla_configurations (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    config JSON,
    version INT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS unla_metrics (
    id INT AUTO_INCREMENT PRIMARY KEY,
    integration_name VARCHAR(255),
    tool_name VARCHAR(255),
    execution_time_ms INT,
    status VARCHAR(50),
    error_message TEXT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert initial configuration
INSERT INTO unla_configurations (name, config) VALUES 
('mcp_srv_mgr', JSON_OBJECT(
    'name', 'mcp_srv_mgr_gateway',
    'version', '1.0.0',
    'enabled', true
))
ON DUPLICATE KEY UPDATE 
config = JSON_OBJECT(
    'name', 'mcp_srv_mgr_gateway',
    'version', '1.0.0',
    'enabled', true
),
updated_at = CURRENT_TIMESTAMP;