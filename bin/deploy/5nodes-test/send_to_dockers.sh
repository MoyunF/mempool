#!/usr/bin/env bash

# 配置部分
#用于编译server，并将程序以及配置文件拷贝到docker中
GO_SOURCE_DIR="../../../server" # Go 源代码路径
OUTPUT_DIR="."           # 编译输出目录
BINARY_NAME="server"   # 可执行文件名
CONFIG_FILE="config.json" # 配置文件名
KILL_FILE="kill_server.sh" #关闭服务器脚本
RUN_FILE="run.sh"  #启动实验脚本
CLEAN="clean_logs.sh" #清空日志脚本
IPS="ips.txt"

DOCKER_CONTAINERS=("mempool_1" "mempool_2" "mempool_3" "mempool_4" "mempool_5") # Docker 容器名称列表
COLLAB_DIR="/collab"      # Docker 中的目标目录

echo "================== Step 1: 编译 Go 程序 =================="

# 进入 Go 源代码目录，编译程序
pushd "$GO_SOURCE_DIR" > /dev/null
GOOS=linux GOARCH=amd64 go build
if [ $? -ne 0 ]; then
  echo "❌ Go 程序编译失败！"
  exit 1
fi
popd > /dev/null

echo "✅ Go 程序编译成功，已生成可执行文件：$OUTPUT_DIR/$BINARY_NAME"

# 检查配置文件是否存在
if [ ! -f "$CONFIG_FILE" ]; then
  echo "❌ 配置文件 $CONFIG_FILE 未找到，请确认！"
  exit 1
fi

echo "================== Step 2: 复制文件到 Docker 容器 =================="

# 遍历所有容器
for container in "${DOCKER_CONTAINERS[@]}"; do
  echo ">>> 处理容器：$container"
  
  # 检查容器是否运行
  if ! docker ps --format '{{.Names}}' | grep -q "^$container$"; then
    echo "⚠️ 容器 $container 未运行，跳过..."
    continue
  fi

  # 创建目标目录（如果不存在）
  docker exec "$container" mkdir -p "$COLLAB_DIR"

  # 复制可执行文件
  docker cp "$GO_SOURCE_DIR/$BINARY_NAME" "$container:$COLLAB_DIR"
  if [ $? -ne 0 ]; then
    echo "❌ 无法复制可执行文件到容器 $container 的 $COLLAB_DIR 目录！"
    exit 1
  fi

  # 复制配置文件
  docker cp "$CONFIG_FILE" "$container:$COLLAB_DIR"
  if [ $? -ne 0 ]; then
    echo "❌ 无法复制配置文件到容器 $container 的 $COLLAB_DIR 目录！"
    exit 1
  fi

  # 复制启动、关闭脚本
  docker cp "$KILL_FILE" "$container:$COLLAB_DIR"
  if [ $? -ne 0 ]; then
    echo "❌ 无法复制杀死服务器到容器 $container 的 $COLLAB_DIR 目录！"
    exit 1
  fi

  docker cp "$RUN_FILE" "$container:$COLLAB_DIR"
  if [ $? -ne 0 ]; then
    echo "❌ 无法复制启动脚本到容器 $container 的 $COLLAB_DIR 目录！"
    exit 1
  fi

  docker cp "$CLEAN" "$container:$COLLAB_DIR"
  if [ $? -ne 0 ]; then
    echo "❌ 无法复制清空日志脚本到容器 $container 的 $COLLAB_DIR 目录！"
    exit 1
  fi

  docker cp "$IPS" "$container:$COLLAB_DIR"
  if [ $? -ne 0 ]; then
    echo "❌ 无法复制清空ip文件到容器 $container 的 $COLLAB_DIR 目录！"
    exit 1
  fi

  echo "✅ 文件成功复制到容器 $container 的 $COLLAB_DIR 目录"


    # 设置文件权限为 777
  docker exec "$container" chmod 777 "$COLLAB_DIR/$BINARY_NAME" "$COLLAB_DIR/$CONFIG_FILE" "$COLLAB_DIR/$KILL_FILE" "$COLLAB_DIR/$RUN_FILE" "$COLLAB_DIR/$CLEAN"
  if [ $? -ne 0 ]; then
    echo "❌ 无法设置容器 $container 中文件的权限！"
    exit 1
  fi
  
  echo "✅ 文件权限设置成功（777）"
done

echo "================== 完成！ =================="
