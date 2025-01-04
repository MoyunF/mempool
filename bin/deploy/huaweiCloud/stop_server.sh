#!/usr/bin/env bash

# 配置部分
IPS="./auth/server_auth.txt"  # 存储 IP 地址的文件
CONTAINER_NAME="mempool_1"    # 容器名称

echo "================== Step 1: 停止服务器 =================="

# 读取每个 IP 地址并执行命令
while IFS= read -r IP; do
  if [ -z "$IP" ]; then
    continue
  fi

  echo "正在连接到服务器: $IP"

  # 检查容器是否存在并运行
  ssh -o StrictHostKeyChecking=no "$IP" "docker ps -q -f name=$CONTAINER_NAME" > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo "❌ 容器 $CONTAINER_NAME 不存在或未运行在 $IP 上"
    continue
  fi

  echo "正在停止容器中的服务器: $CONTAINER_NAME"

  # 通过 SSH 执行停止服务器的命令
  ssh -o StrictHostKeyChecking=no "$IP" "docker exec $CONTAINER_NAME sh -c 'cd /collab && bash ./kill_server.sh'" > /dev/null 2>&1
  
  if [ $? -ne 0 ]; then
    echo "❌ 无法在 $IP 上停止服务器"
    continue
  fi

  echo "✅ 服务器已在 $IP 上停止"

done < "$IPS"

echo "================== 完成！ =================="
