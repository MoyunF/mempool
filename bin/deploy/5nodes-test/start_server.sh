#!/bin/bash

# 检查是否提供了参数
if [ -z "$1" ]; then
  echo "请提供本次实验名称"
  exit 1
fi

# 获取所有容器的 ID 和名称
CONTAINERS=$(docker ps --format "{{.ID}} {{.Names}}")

# 如果没有容器运行
if [ -z "$CONTAINERS" ]; then
  echo "没有正在运行的容器。"
  exit 1
fi

# 初始化 id 计数器
ID=1

# 获取实验 ID
EXP_ID=$1

# 遍历容器并执行命令
while read -r CONTAINER_INFO; do
  CONTAINER_ID=$(echo "$CONTAINER_INFO" | awk '{print $1}')
  CONTAINER_NAME=$(echo "$CONTAINER_INFO" | awk '{print $2}')

  # 仅处理容器名称以 mempool 开头的
  if [[ $CONTAINER_NAME == mempool* ]]; then
    echo "在容器 $CONTAINER_NAME ($CONTAINER_ID) 中执行命令，id=$ID"

    # 执行命令并捕获输出和错误
    OUTPUT=$(docker exec "$CONTAINER_ID" sh -c "cd /collab && nohup ./run.sh $EXP_ID" 2>&1)
    EXIT_CODE=$?

    # 根据命令返回值判断是否成功
    if [ $EXIT_CODE -ne 0 ]; then
      echo "容器 $CONTAINER_NAME ($CONTAINER_ID) 中的命令执行失败。"
      echo "错误信息："
      echo "$OUTPUT"
      continue
    fi

    echo "容器 $CONTAINER_NAME ($CONTAINER_ID) 中命令执行成功。"
    echo "输出信息："
    echo "$OUTPUT"

    # 递增 ID
    ID=$((ID + 1))
  else
    echo "跳过容器 $CONTAINER_NAME ($CONTAINER_ID)，因为名称不匹配。"
  fi

done <<< "$CONTAINERS"

echo "所有容器命令执行完成。"
