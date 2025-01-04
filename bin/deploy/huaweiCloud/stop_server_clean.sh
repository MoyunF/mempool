#!/usr/bin/env bash

# 配置部分
IPS="./auth/server_auth.txt"  # 存储 IP 地址的文件
PARAM=$1               # 接受的参数，用于传递给 clean_logs.sh
CONTAINER_NAME="mempool_1"

if [ -z "$PARAM" ]; then
  echo "❌ 需要提供参数！"
  exit 1
fi

echo "================== Step 1: 停止服务器并清理日志 =================="

# 读取每个 IP 地址并执行命令
while IFS= read -r IP; do
  if [ -z "$IP" ]; then
    continue
  fi

  echo "正在连接到服务器: $IP"


  echo "正在清理容器: $CONTAINER_NAME"


  ssh -o StrictHostKeyChecking=no "$IP" "docker exec $CONTAINER_NAME sh -c 'cd /collab && bash ./clean_logs.sh $PARAM'" > /dev/null 2>&1
  
  if [ $? -ne 0 ]; then
    echo "❌ 无法在 $IP 上执行 clean_logs.sh"
    continue
  fi

  echo "✅ 在 $IP 上清理日志完成"

done < "$IPS"

echo "================== 完成！ =================="
