#!/bin/bash

# 获取所有容器的 ID 和名称
CONTAINERS=$(docker ps --format "{{.ID}} {{.Names}}")

# 如果没有容器运行
if [ -z "$CONTAINERS" ]; then
  echo "没有正在运行的容器。"
  exit 1
fi

# 初始化 id 计数器
ID=1

# 遍历容器并执行命令
while read -r CONTAINER_INFO; do
  {
    CONTAINER_ID=$(echo "$CONTAINER_INFO" | awk '{print $1}')
    CONTAINER_NAME=$(echo "$CONTAINER_INFO" | awk '{print $2}')

    # 仅处理容器名称以 mempool 开头的
    if [[ $CONTAINER_NAME == mempool* ]]; then
      echo "在容器 $CONTAINER_NAME ($CONTAINER_ID) 中执行命令"

      docker exec "$CONTAINER_ID" sh -c "cd /collab && bash ./kill_server.sh"

      # 检查命令执行是否成功
      if [ $? -ne 0 ]; then
        echo "容器 $CONTAINER_NAME ($CONTAINER_ID) 中的命令执行失败，跳过剩余操作。"
        continue
      fi

      echo "容器 $CONTAINER_NAME ($CONTAINER_ID) 中命令执行成功。"

      
    else
      echo "跳过容器 $CONTAINER_NAME ($CONTAINER_ID)，因为名称不匹配。"
    fi
  }

done <<< "$CONTAINERS"
wait

echo "所有容器命令执行完成。"
