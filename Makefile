# Linux服务管理器 Makefile

# 变量定义
BINARY_NAME=mcp-server
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION=$(shell go version | awk '{print $$3}')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GoVersion=$(GO_VERSION)"

# 默认目标
.DEFAULT_GOAL := help

# 颜色定义
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
BLUE=\033[0;34m
NC=\033[0m # No Color

.PHONY: help
help: ## 显示帮助信息
	@echo "Linux服务管理器 - 可用命令:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(BLUE)%-20s$(NC) %s\n", $$1, $$2}'

.PHONY: deps
deps: ## 安装依赖
	@echo "$(BLUE)安装Go依赖...$(NC)"
	go mod download
	go mod tidy
	@echo "$(GREEN)依赖安装完成$(NC)"

.PHONY: build
build: deps ## 构建应用
	@echo "$(BLUE)构建应用...$(NC)"
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/server
	@echo "$(GREEN)构建完成: $(BINARY_NAME)$(NC)"

.PHONY: build-all
build-all: deps ## 构建所有平台的二进制文件
	@echo "$(BLUE)构建所有平台...$(NC)"
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/server
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/server
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/server
	@echo "$(GREEN)所有平台构建完成$(NC)"

.PHONY: test
test: ## 运行单元测试
	@echo "$(BLUE)运行单元测试...$(NC)"
	./test.sh -u -v

.PHONY: test-all
test-all: ## 运行所有测试
	@echo "$(BLUE)运行所有测试...$(NC)"
	./test.sh -a -v

.PHONY: test-integration
test-integration: ## 运行集成测试
	@echo "$(BLUE)运行集成测试...$(NC)"
	./test.sh -i -v

.PHONY: test-coverage
test-coverage: ## 生成测试覆盖率报告
	@echo "$(BLUE)生成覆盖率报告...$(NC)"
	./test.sh -c -v

.PHONY: benchmark
benchmark: ## 运行性能测试
	@echo "$(BLUE)运行性能测试...$(NC)"
	./test.sh -b -v

.PHONY: lint
lint: ## 运行代码检查
	@echo "$(BLUE)运行代码检查...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "$(YELLOW)golangci-lint未安装，使用基本检查$(NC)"; \
		go vet ./...; \
		gofmt -l .; \
	fi

.PHONY: fmt
fmt: ## 格式化代码
	@echo "$(BLUE)格式化代码...$(NC)"
	go fmt ./...
	@echo "$(GREEN)代码格式化完成$(NC)"

.PHONY: clean
clean: ## 清理构建文件
	@echo "$(BLUE)清理文件...$(NC)"
	rm -f $(BINARY_NAME)
	rm -rf dist/
	./test.sh --clean
	@echo "$(GREEN)清理完成$(NC)"

.PHONY: run
run: build ## 构建并运行HTTP模式
	@echo "$(BLUE)启动HTTP服务器...$(NC)"
	./$(BINARY_NAME) -config config.yaml

.PHONY: run-mcp
run-mcp: build ## 构建并运行MCP模式
	@echo "$(BLUE)启动MCP服务器...$(NC)"
	./$(BINARY_NAME) -mcp -log-level debug

.PHONY: dev
dev: ## 启动开发环境
	@echo "$(BLUE)启动开发环境...$(NC)"
	docker-compose --profile dev up --build

.PHONY: docker-build
docker-build: ## 构建Docker镜像
	@echo "$(BLUE)构建Docker镜像...$(NC)"
	docker build -t linux-service-manager:$(VERSION) .
	docker tag linux-service-manager:$(VERSION) linux-service-manager:latest
	@echo "$(GREEN)Docker镜像构建完成$(NC)"

.PHONY: docker-test
docker-test: ## 在Docker中运行测试
	@echo "$(BLUE)在Docker中运行测试...$(NC)"
	docker-compose --profile test up --build --abort-on-container-exit

.PHONY: docker-run
docker-run: docker-build ## 运行Docker容器
	@echo "$(BLUE)启动Docker容器...$(NC)"
	docker-compose up -d

.PHONY: docker-stop
docker-stop: ## 停止Docker容器
	@echo "$(BLUE)停止Docker容器...$(NC)"
	docker-compose down

.PHONY: install
install: build ## 安装到系统
	@echo "$(BLUE)安装到系统...$(NC)"
	sudo cp $(BINARY_NAME) /usr/local/bin/
	@echo "$(GREEN)安装完成$(NC)"

.PHONY: uninstall
uninstall: ## 从系统卸载
	@echo "$(BLUE)从系统卸载...$(NC)"
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "$(GREEN)卸载完成$(NC)"

.PHONY: release
release: ## 创建发布版本
	@echo "$(BLUE)创建发布版本...$(NC)"
	@if [ -z "$(TAG)" ]; then \
		echo "$(RED)请指定TAG: make release TAG=v1.0.0$(NC)"; \
		exit 1; \
	fi
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)
	@echo "$(GREEN)发布标签 $(TAG) 已创建$(NC)"

.PHONY: docs
docs: ## 启动文档服务器
	@echo "$(BLUE)启动文档服务器...$(NC)"
	docker-compose --profile docs up -d
	@echo "$(GREEN)文档服务器已启动: http://localhost:8081$(NC)"

.PHONY: security-scan
security-scan: ## 运行安全扫描
	@echo "$(BLUE)运行安全扫描...$(NC)"
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "$(YELLOW)gosec未安装，跳过安全扫描$(NC)"; \
	fi

.PHONY: mod-update
mod-update: ## 更新Go模块
	@echo "$(BLUE)更新Go模块...$(NC)"
	go get -u all
	go mod tidy
	@echo "$(GREEN)模块更新完成$(NC)"

.PHONY: version
version: ## 显示版本信息
	@echo "版本: $(VERSION)"
	@echo "构建时间: $(BUILD_TIME)"
	@echo "Go版本: $(GO_VERSION)"

.PHONY: info
info: ## 显示项目信息
	@echo "$(BLUE)项目信息:$(NC)"
	@echo "  名称: Linux服务管理器"
	@echo "  版本: $(VERSION)"
	@echo "  二进制: $(BINARY_NAME)"
	@echo "  Go版本: $(GO_VERSION)"
	@echo ""
	@echo "$(BLUE)可用服务管理器:$(NC)"
	@if command -v systemctl >/dev/null 2>&1; then echo "  ✅ systemd"; else echo "  ❌ systemd"; fi
	@if [ -d "/etc/init.d" ]; then echo "  ✅ SysV init"; else echo "  ❌ SysV init"; fi
	@if command -v docker >/dev/null 2>&1; then echo "  ✅ Docker"; else echo "  ❌ Docker"; fi

# 快捷方式
.PHONY: t tc tb tr
t: test ## test的快捷方式
tc: test-coverage ## test-coverage的快捷方式  
tb: benchmark ## benchmark的快捷方式
tr: test-all ## test-all的快捷方式