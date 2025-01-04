#!/bin/bash

# 检查是否提供了命令作为参数
if [ "$#" -eq 0 ]; then
  echo "用法: $0 \"<要执行的命令>\""
  exit 1
fi

# 获取传入的命令
COMMAND="$1"

# 获取所有容器的 ID 和名称
CONTAINERS=$(docker ps --format "{{.ID}} {{.Names}}")

# 如果没有容器运行
if [ -z "$CONTAINERS" ]; then
  echo "没有正在运行的容器。"
  exit 1
fi

while read -r CONTAINER_INFO; do
  CONTAINER_ID=$(echo "$CONTAINER_INFO" | awk '{print $1}')
  CONTAINER_NAME=$(echo "$CONTAINER_INFO" | awk '{print $2}')

  # 仅处理容器名称以 mempool 开头的
  if [[ $CONTAINER_NAME == mempool* ]]; then
    # 遍历容器并执行命令
      echo "执行命令: $COMMAND 到容器: $CONTAINER_NAME"
      docker exec "$CONTAINER_ID" sh -c /bin/bash -c "cd /collab && $COMMAND"
  fi
done <<< "$CONTAINERS"
echo "所有容器命令执行完成。"



