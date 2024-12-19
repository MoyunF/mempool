#!/bin/bash

# 检查是否提供了参数
if [ -z "$1" ]; then
  echo "清楚输入本次实验名称"
  exit 1
fi

# 获取所有容器的 ID
CONTAINERS=$(docker ps -q)

# 如果没有容器运行
if [ -z "$CONTAINERS" ]; then
  echo "没有正在运行的容器。"
  exit 1
fi

# 初始化 id 计数器
ID=1

#
EXP_ID=$1

# 遍历容器并执行命令
for CONTAINER in $CONTAINERS; do
  echo "在容器 $CONTAINER 中执行命令，id=$ID"

  docker exec "$CONTAINER" sh -c "cd /collab && nohup ./run.sh $ID $EXP_ID > /dev/null 2>&1 &"

  # 检查命令执行是否成功
  if [ $? -ne 0 ]; then
    echo "容器 $CONTAINER 中的命令执行失败，跳过剩余操作。"
    continue
  fi

  echo "容器 $CONTAINER 中命令执行成功。"

  # 递增 ID
  ID=$((ID + 1))
done

echo "所有容器命令执行完成。"
