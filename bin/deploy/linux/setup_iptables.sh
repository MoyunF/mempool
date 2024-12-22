#!/usr/bin/env bash
DEPLOY_NAME=$(jq '.auth.user' service_conf.json | sed 's/\"//g')
DEPLOY_FILE=$(jq '.server.dir' service_conf.json | sed 's/\"//g')
DEPLOY_IPS_FILE=$(jq '.server.deploy_file' service_conf.json | sed 's/\"//g')

./pkill_server.sh
distribute(){
    for line in $(cat $DEPLOY_IPS_FILE)
    do
      ssh $DEPLOY_NAME@$line "mkdir ~/$DEPLOY_FILE"&
    done
    wait
    echo "Uploading binaries..."
    for line in $(cat $DEPLOY_IPS_FILE)
    do
      scp iptables.sh $DEPLOY_NAME@$line:~/$DEPLOY_FILE&
    done
    wait
    echo "Upload success!"
}

# distribute files
distribute


# 获取所有正在运行的Docker容器的ID
container_ids=$(docker ps -q)

# 遍历每个容器ID并执行iptables.sh脚本
for container_id in $container_ids; do
  echo "在容器 $container_id 中执行 iptables.sh 脚本"
  docker exec "$container_id" bash -c "sudo bash ~/bamboo/iptables.sh"
done
