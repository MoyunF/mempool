#!/bin/bash

#用来启动实验需要的docker容器

# Docker镜像名称
IMAGE_NAME="liuxin2629/mempool:latest"

# Docker网络名称,需先创建docker网络
NETWORK_NAME="exp"

# 起始IP地址 (根据你的网络配置调整)
BASE_IP="172.18.0."
START_IP=2

# 容器数量
CONTAINER_COUNT=5

# 创建容器
for i in $(seq 1 $CONTAINER_COUNT); do
    CONTAINER_NAME="mempool_$i"
    CONTAINER_IP="${BASE_IP}$((START_IP + i - 1))"

    echo "正在启动容器: $CONTAINER_NAME, 分配IP: $CONTAINER_IP"

    docker run -d --rm \
        --name $CONTAINER_NAME \
        --network $NETWORK_NAME \
        --ip $CONTAINER_IP \
        --privileged \
        -it \
        $IMAGE_NAME \
        /bin/bash -c "export TERM=xterm && service ssh start && top"

    if [ $? -ne 0 ]; then
        echo "容器 $CONTAINER_NAME 启动失败!"
        exit 1
    fi
done

echo "所有容器已启动!"
docker ps
