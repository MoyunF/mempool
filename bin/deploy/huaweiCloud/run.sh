#!/usr/bin/env bash
#用来在主机中启动collab主程序

SERVER_PID_FILE=server.pid

# 获取 IP 地址
ip=$(ifconfig | grep -oP 'inet 172\.\d+\.\d+\.\d+' | awk '{print $2}')
echo "Detected IP: $ip"

# 提取 IP 的最后一个字段
last_field=$(echo "$ip" | awk -F. '{print $NF}')

# 检查是否为数字，并计算 x-1
if [[ $last_field =~ ^[0-9]+$ ]]; then
    new_value=$((last_field - 1))
    echo "Node id: $new_value"
else
    echo "The last ip address is not a valid number: $last_field"
    exit 1
fi

if [ -z "${SERVER_PID}" ]; then
    mkdir ./logs/$1
    ./server -id $new_value -log_dir=./logs/$1 -log_level=INFO -log_id=$new_value -algorithm=hotstuff > program.log 2>&1 &
    echo $! >> ${SERVER_PID_FILE}
    echo "collab启动！"
else
    echo "Servers are already started in this folder."
    exit 0
fi
