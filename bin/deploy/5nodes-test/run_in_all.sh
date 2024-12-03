#!/bin/bash

# 检查是否提供了命令作为参数
if [ "$#" -eq 0 ]; then
  echo "用法: $0 \"<要执行的命令>\""
  exit 1
fi

# 获取传入的命令
COMMAND="$1"

# 获取所有容器的 ID
CONTAINERS=$(docker ps -q)

# 如果没有容器运行
if [ -z "$CONTAINERS" ]; then
  echo "没有正在运行的容器。"
  exit 1
fi

# 遍历容器并执行命令
for CONTAINER in $CONTAINERS; do
  echo "执行命令到容器: $CONTAINER"
  docker exec -it "$CONTAINER" /bin/bash -c "cd /collab && $COMMAND"
done
