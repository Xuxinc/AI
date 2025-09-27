# AI Celebrity Simulator - Go Backend Makefile
# 项目管理脚本

# 应用名称
APP_NAME := ai-celebrity-simulator

# Go相关变量
MAIN_FILE := main.go
BUILD_DIR := build
BINARY_NAME := $(APP_NAME)

# 颜色输出
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m # No Color

.PHONY: help install build run clean fmt vet deps start stop restart status

# 默认目标
.DEFAULT_GOAL := help

## 显示帮助信息
help:
	@echo "$(GREEN)AI Celebrity Simulator - Go Backend$(NC)"
	@echo "$(YELLOW)可用命令:$(NC)"
	@echo "  $(GREEN)install$(NC)     - 安装依赖包"
	@echo "  $(GREEN)build$(NC)       - 构建应用程序"
	@echo "  $(GREEN)run$(NC)         - 运行应用程序（前台运行）"
	@echo "  $(GREEN)start$(NC)       - 启动服务（后台运行）"
	@echo "  $(GREEN)stop$(NC)        - 停止服务"
	@echo "  $(GREEN)restart$(NC)     - 重启服务"
	@echo "  $(GREEN)status$(NC)      - 查看服务状态"
	@echo "  $(GREEN)clean$(NC)       - 清理文件和日志"
	@echo "  $(GREEN)vet$(NC)         - 代码静态检查"
	@echo "  $(GREEN)deps$(NC)        - 更新依赖"

## 安装依赖包
install:
	@echo "$(YELLOW)安装依赖包...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)✓ 依赖安装完成$(NC)"

## 构建应用程序
build: install fmt vet
	@echo "$(YELLOW)构建应用程序...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "$(GREEN)✓ 构建完成: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

## 运行应用程序
run:
	@echo "$(YELLOW)启动 AI Celebrity Simulator 服务...$(NC)"
	@if [ -f $(BUILD_DIR)/$(BINARY_NAME) ]; then \
		echo "$(GREEN)使用已构建的二进制文件运行...$(NC)"; \
		./$(BUILD_DIR)/$(BINARY_NAME); \
	else \
		echo "$(GREEN)直接运行源码...$(NC)"; \
		go run $(MAIN_FILE); \
	fi


## 代码静态检查
vet:
	@echo "$(YELLOW)执行代码静态检查...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✓ 静态检查通过$(NC)"

## 更新依赖
deps:
	@echo "$(YELLOW)更新依赖...$(NC)"
	@go mod tidy
	@go mod download
	@echo "$(GREEN)✓ 依赖更新完成$(NC)"

## 清理构建文件和日志
clean:
	@echo "$(YELLOW)清理构建文件和日志...$(NC)"
	@rm -rf $(BUILD_DIR)
	@if [ -d "logs" ]; then \
		rm -rf logs/*.log; \
		echo "$(GREEN)✓ 日志文件已清理$(NC)"; \
	fi
	@go clean
	@echo "$(GREEN)✓ 清理完成$(NC)"

## 启动服务（后台运行）
start:
	@echo "$(YELLOW)启动 AI Celebrity Simulator 服务...$(NC)"
	@chmod +x scripts/start.sh
	@./scripts/start.sh

## 停止服务
stop:
	@echo "$(YELLOW)停止 AI Celebrity Simulator 服务...$(NC)"
	@chmod +x scripts/stop.sh
	@./scripts/stop.sh

## 重启服务
restart:
	@echo "$(YELLOW)重启 AI Celebrity Simulator 服务...$(NC)"
	@chmod +x scripts/restart.sh
	@./scripts/restart.sh

## 查看服务状态
status:
	@echo "$(YELLOW)查看 AI Celebrity Simulator 服务状态...$(NC)"
	@chmod +x scripts/status.sh
	@./scripts/status.sh 