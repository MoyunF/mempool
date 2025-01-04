#!/usr/bin/env bash

# 配置部分
# 用于编译 server，并将程序以及配置文件拷贝到 docker 中
GO_SOURCE_DIR="../../../server"  # Go 源代码路径
OUTPUT_DIR="."                   # 编译输出目录
BINARY_NAME="server"             # 可执行文件名
CONFIG_FILE="config.json"        # 配置文件名
KILL_FILE="kill_server.sh"       # 关闭服务器脚本
RUN_FILE="run.sh"                # 启动实验脚本
CLEAN="clean_logs.sh"            # 清空日志脚本
IPS="./auth/server_auth.txt"            # 存储 IP 地址的文件


echo "================== Step 1: 编译 Go 程序 =================="

# 进入 Go 源代码目录，编译程序
pushd "$GO_SOURCE_DIR" > /dev/null
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build
if [ $? -ne 0 ]; then
  echo "❌ Go 程序编译失败！"
  exit 1
fi
popd > /dev/null

echo "✅ Go 程序编译成功，已生成可执行文件：$OUTPUT_DIR/$BINARY_NAME"

echo "================== Step 2: 传输文件到远程服务器 =================="

# 读取每个 IP 地址并传输文件
while IFS= read -r IP; do
  if [ -z "$IP" ]; then
    continue
  fi

  echo "正在将文件传输到服务器: $IP"

  # 传输文件到远程服务器
  scp "$OUTPUT_DIR/$BINARY_NAME" "$CONFIG_FILE" "$KILL_FILE" "$RUN_FILE" "$CLEAN" "$IP:/root/mempool/bin/deploy/huaweiCloud" 
  if [ $? -ne 0 ]; then
    echo "❌ 文件传输失败到 $IP"
    continue
  fi

  echo "✅ 文件成功传输到 $IP"

  # 设置文件权限为 777
  echo "正在设置文件权限为 777"
  ssh "$IP" "chmod -R 777 /root/mempool/bin/deploy/huaweiCloud"
  if [ $? -ne 0 ]; then
    echo "❌ 设置权限失败到 $IP"
    continue
  fi

  echo "✅ 权限设置成功"
done < "$IPS"

echo "================== 完成！ =================="
