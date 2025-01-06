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
COLLAB_DIR="."      # Docker 中的目标目录

echo "================== Step 1: 编译 Go 程序 =================="

# 进入 Go 源代码目录，编译程序
pushd "$GO_SOURCE_DIR" > /dev/null
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 GOFLAGs="-race" go build -o ../bin/deploy/5nodes-test/server
if [ $? -ne 0 ]; then
  echo "❌ Go 程序编译失败！"
  exit 1
fi
popd > /dev/null

echo "✅ Go 程序编译成功，已生成可执行文件：$OUTPUT_DIR/$BINARY_NAME"
    # 设置文件权限为 777
  chmod 777 "$COLLAB_DIR/$BINARY_NAME" "$COLLAB_DIR/$CONFIG_FILE" "$COLLAB_DIR/$KILL_FILE" "$COLLAB_DIR/$RUN_FILE" "$COLLAB_DIR/$CLEAN"
  if [ $? -ne 0 ]; then
    echo "❌ 无法设置文件的权限！"
    exit 1
  fi
  
  echo "✅ 文件权限设置成功（777）"

echo "================== 完成！ =================="
