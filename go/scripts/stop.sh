#!/bin/bash

# AI Celebrity Simulator 停止服务脚本
APP_NAME="ai-celebrity-simulator"
PID_FILE="build/ai-celebrity-simulator.pid"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# 检查服务是否运行
is_service_running() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            return 0
        else
            rm -f "$PID_FILE"
            return 1
        fi
    fi
    return 1
}

# 优雅关闭服务
stop_service() {
    echo -e "${YELLOW}🛑 停止 $APP_NAME 服务...${NC}"
    
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            echo -e "${YELLOW}📡 发送SIGTERM信号到进程 $pid${NC}"
            kill -15 "$pid"
            
            # 等待进程优雅关闭（最多等待30秒）
            local count=0
            while kill -0 "$pid" 2>/dev/null && [ $count -lt 30 ]; do
                sleep 1
                count=$((count + 1))
                if [ $((count % 5)) -eq 0 ]; then
                    echo -e "${YELLOW}⏳ 等待进程关闭... (${count}s)${NC}"
                fi
            done
            
            # 检查进程是否已关闭
            if kill -0 "$pid" 2>/dev/null; then
                echo -e "${YELLOW}⚠️ 进程未在30秒内关闭，发送SIGKILL信号${NC}"
                kill -9 "$pid"
                sleep 2
            fi
            
            # 最终检查
            if kill -0 "$pid" 2>/dev/null; then
                echo -e "${RED}❌ 无法关闭进程 $pid${NC}"
                return 1
            else
                echo -e "${GREEN}✅ 进程 $pid 已成功关闭${NC}"
                rm -f "$PID_FILE"
                return 0
            fi
        else
            echo -e "${YELLOW}⚠️ PID文件存在但进程 $pid 不存在，清理PID文件${NC}"
            rm -f "$PID_FILE"
            return 0
        fi
    else
        echo -e "${YELLOW}ℹ️ 未找到PID文件，服务可能未运行${NC}"
        return 0
    fi
}

# 主函数
main() {
    # 检查服务状态
    if ! is_service_running; then
        echo -e "${YELLOW}ℹ️ 服务未运行${NC}"
        exit 0
    fi
    
    # 停止服务
    if stop_service; then
        echo -e "${GREEN}🎉 服务停止成功！${NC}"
    else
        echo -e "${RED}❌ 服务停止失败${NC}"
        exit 1
    fi
}

main "$@"
