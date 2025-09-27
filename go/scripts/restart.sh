#!/bin/bash

# AI Celebrity Simulator 重启脚本
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

# 启动服务
start_service() {
    # 检查是否已构建
    if [ ! -f "build/$APP_NAME" ]; then
        echo -e "${YELLOW}🔨 二进制文件不存在，开始构建...${NC}"
        make build
        if [ $? -ne 0 ]; then
            echo -e "${RED}❌ 构建失败${NC}"
            return 1
        fi
    fi
    
    # 启动服务
    echo -e "${YELLOW}▶️ 启动服务进程...${NC}"
    nohup ./build/$APP_NAME > /dev/null 2>&1 &
    local new_pid=$!
    
    # 保存PID
    echo "$new_pid" > "$PID_FILE"
    echo -e "${GREEN}📝 服务PID: $new_pid${NC}"
    
    # 等待服务启动
    sleep 3
    
    # 检查服务是否成功启动
    if kill -0 "$new_pid" 2>/dev/null; then
        echo -e "${GREEN}✅ 服务启动成功${NC}"
        return 0
    else
        echo -e "${RED}❌ 服务启动失败${NC}"
        rm -f "$PID_FILE"
        return 1
    fi
}

# 主函数
main() {
    echo -e "${YELLOW}🔄 重启 $APP_NAME 服务...${NC}"
    
    # 停止服务
    if ! stop_service; then
        echo -e "${RED}❌ 停止服务失败，重启终止${NC}"
        exit 1
    fi
    
    # 等待一下确保端口释放
    sleep 2
    
    # 启动服务
    if ! start_service; then
        echo -e "${RED}❌ 启动服务失败，重启终止${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}🎉 服务重启完成！${NC}"
    echo -e "${GREEN}📊 服务状态: 运行中 (PID: $(cat "$PID_FILE"))${NC}"
    echo -e "${GREEN}📁 应用日志: logs/ai-celebrity-simulator_all.log${NC}"
}

main "$@"
