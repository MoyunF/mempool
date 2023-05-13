#!/bin/bash

# 获取所有正在运行的容器ID
container_ids=$(docker ps -q)

# 循环遍历每个容器ID并停止容器
for container_id in $container_ids; do
    echo "Stopping container: $container_id"
    docker stop $container_id
done

echo "All running containers have been stopped."
