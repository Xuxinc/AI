#!/bin/bash

# AI Celebrity Simulator 服务状态检查脚本
APP_NAME="ai-celebrity-simulator"
PID_FILE="build/ai-celebrity-simulator.pid"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

check_status() {
    echo -e "${BLUE}🔍 检查 $APP_NAME 服务状态...${NC}"
    
    if [ ! -f "$PID_FILE" ]; then
        echo -e "${YELLOW}📋 状态: 未运行${NC}"
        echo -e "${YELLOW}💡 启动服务: make start${NC}"
        return 0  # 未运行是正常状态，不应该返回错误码
    fi
    
    local pid=$(cat "$PID_FILE")
    if kill -0 "$pid" 2>/dev/null; then
        echo -e "${GREEN}✅ 状态: 运行中${NC}"
        echo -e "${GREEN}📊 PID: $pid${NC}"
        echo -e "${GREEN}💡 停止服务: make stop${NC}"
        echo -e "${GREEN}🔄 重启服务: make restart${NC}"
        
        # 显示进程信息
        echo -e "${BLUE}📋 进程信息:${NC}"
        if ps -p "$pid" -o pid,ppid,etime,pcpu,pmem,command 2>/dev/null; then
            echo -e "${GREEN}✓ 进程信息获取成功${NC}"
        else
            echo -e "${YELLOW}⚠️ 无法获取详细进程信息，但进程正在运行${NC}"
        fi
        
        # 显示进程启动时间（macOS兼容）
        if command -v psutil >/dev/null 2>&1; then
            # 如果有psutil，使用Python获取启动时间
            local start_time=$(python3 -c "import psutil; print(psutil.Process($pid).create_time())" 2>/dev/null)
            if [ -n "$start_time" ]; then
                local start_date=$(date -r "$start_time" '+%Y-%m-%d %H:%M:%S' 2>/dev/null)
                echo -e "${BLUE}🕐 启动时间: $start_date${NC}"
            fi
        else
            # 使用ps命令的etime字段（相对时间）
            local etime=$(ps -p "$pid" -o etime= 2>/dev/null)
            if [ -n "$etime" ]; then
                echo -e "${BLUE}⏱️ 运行时长: $etime${NC}"
            fi
        fi
        
        return 0
    else
        echo -e "${RED}❌ 状态: 异常 (PID文件存在但进程不存在)${NC}"
        echo -e "${YELLOW}🧹 清理PID文件...${NC}"
        rm -f "$PID_FILE"
        echo -e "${YELLOW}💡 启动服务: make start${NC}"
        return 1
    fi
}

check_status
