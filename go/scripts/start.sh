#!/bin/bash

# AI Celebrity Simulator 启动服务脚本
APP_NAME="ai-celebrity-simulator"
PID_FILE="build/ai-celebrity-simulator.pid"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# 检查服务是否已运行
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

# 启动服务
start_service() {
    echo -e "${YELLOW}🚀 启动 $APP_NAME 服务...${NC}"
    
    # 检查服务是否已运行
    if is_service_running; then
        echo -e "${YELLOW}⚠️ 服务已在运行 (PID: $(cat "$PID_FILE"))${NC}"
        echo -e "${YELLOW}💡 如需重启，请使用: make restart${NC}"
        return 1
    fi
    
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
    echo -e "${YELLOW}⏳ 等待服务启动...${NC}"
    sleep 3
    
    # 检查服务是否成功启动
    if kill -0 "$new_pid" 2>/dev/null; then
        echo -e "${GREEN}✅ 服务启动成功${NC}"
        echo -e "${GREEN}📊 服务状态: 运行中${NC}"
        echo -e "${GREEN}📁 应用日志: logs/ai-celebrity-simulator_all.log${NC}"
        echo -e "${GREEN}🔍 查看日志: tail -f logs/ai-celebrity-simulator_all.log${NC}"
        return 0
    else
        echo -e "${RED}❌ 服务启动失败${NC}"
        rm -f "$PID_FILE"
        return 1
    fi
}

# 主函数
main() {
    # 启动服务
    if start_service; then
        echo -e "${GREEN}🎉 服务启动完成！${NC}"
    else
        echo -e "${RED}❌ 服务启动失败${NC}"
        exit 1
    fi
}

main "$@"
