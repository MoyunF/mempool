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
CONTAINER_COUNT=1

# 挂载的端口号
HTTP_PORT=8070
# collab communication port
COLLAB_PORT=3735
COLLAB_PORT2=2629

# 创建容器
for i in $(seq 1 $CONTAINER_COUNT); do
    CONTAINER_NAME="mempool_$i"
    CONTAINER_IP="${BASE_IP}$((START_IP + i - 1))"

    echo "正在启动容器: $CONTAINER_NAME, 分配IP: $CONTAINER_IP, 挂载端口：$((HTTP_PORT+i-1))、$((COLLAB_PORT+i-1))、$((COLLAB_PORT2+i-1))"

    docker run -d --rm \
        --name $CONTAINER_NAME \
        --network $NETWORK_NAME \
        --ip $CONTAINER_IP \
        --privileged \
        -it \
        -p $((HTTP_PORT+i-1)):8070 \
	    -p $((COLLAB_PORT+i-1)):3735 \
        -p $((COLLAB_PORT2+i-1)):2629 \
       	-v /root/logs:/collab/logs \
	    -v /root/mempool/bin/deploy/huaweiCloud:/collab \
        $IMAGE_NAME \
        /bin/bash -c "export TERM=xterm && service ssh start && top"

    if [ $? -ne 0 ]; then
        echo "容器 $CONTAINER_NAME 启动失败!"
        exit 1
    fi
done

echo "所有容器已启动!"
docker ps
