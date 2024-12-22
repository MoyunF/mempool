#!/bin/bash

# docker run -dit -p 8070:8070 --rm --network exp1 --ip 192.168.0.2 --mac-address 02:42:ac:11:00:02 --name container1 mempool
# docker run -dit -p 8071:8070 --rm --network exp1 --ip 192.168.0.3 --mac-address 02:42:ac:11:00:03 --name container2 mempool
# docker run -dit -p 8072:8070 --rm --network exp1 --ip 192.168.0.4 --mac-address 02:42:ac:11:00:04 --name container3 mempool
# docker run -dit -p 8073:8070 --rm --network exp1 --ip 192.168.0.5 --mac-address 02:42:ac:11:00:05 --name container4 mempool

# docker run -dit --network exp1 --network-bandwidth 10m --ip 192.168.0.12 --mac-address 02:42:ac:11:00:12 --name container11 mempool sh -c "service ssh start && top"
# docker run -dit --network exp1 --network-bandwidth 10m --ip 192.168.0.13 --mac-address 02:42:ac:11:00:13 --name container12 mempool sh -c "service ssh start && top"
# docker run -dit --network -bandwidth 10m exp1 --ip 192.168.0.14 --mac-address 02:42:ac:11:00:14 --name container13 mempool sh -c "service ssh start && top"
# docker run -dit --network -bandwidth 10m exp1 --ip 192.168.0.15 --mac-address 02:42:ac:11:00:15 --name container14 mempool sh -c "service ssh start && top"

# docker run -dit --network -bandwidth 10m exp1 --ip 192.168.0.16 --mac-address 02:42:ac:11:00:17 --name container15 mempool sh -c "service ssh start && top"
# docker run -dit --network -bandwidth 10m exp1 --ip 192.168.0.17 --mac-address 02:42:ac:11:00:18 --name container16 mempool sh -c "service ssh start && top"
# docker run -dit --network -bandwidth 10m exp1 --ip 192.168.0.18 --mac-address 02:42:ac:11:00:19 --name client1 mempool sh -c "service ssh start && top"
# docker run -dit --network -bandwidth 10m exp1 --ip 192.168.0.19 --mac-address 02:42:ac:11:00:20 --name client2 mempool sh -c "service ssh start && top"


#!/bin/bash

# 指定要启动的Docker容器数量
container_count=$1

# 设置初始IP地址和MAC地址
ip_address="192.168.0.2"
mac_address="00:00:00:00:10:00"

# 设置初始容器索引
container_index=1
bandwidth_limit="102400"

# 清空现有的 public_ips.txt 文件
> public_ips.txt

# 检查容器数量是否为空或未定义
if [ -z "$container_count" ]; then
  echo "请提供要启动的容器数量作为参数"
  exit 1
fi

# 循环启动Docker容器
while [ $container_index -le $container_count ]; do
  # 递增IP地址和MAC地址
  ip_address_parts=($(echo $ip_address | tr '.' ' '))
  ip_address_parts[3]=$((ip_address_parts[3]+1))
  ip_address=$(echo ${ip_address_parts[@]} | tr ' ' '.')

  mac_address_parts=($(echo $mac_address | tr ':' ' '))
  mac_address_parts[5]=$((16#${mac_address_parts[5]}+1))
  mac_address=$(printf "%02x:" "${mac_address_parts[@]}")
  mac_address=${mac_address%:}

  # 递增容器名称
  container_name="client$container_index"
  # 启动Docker容器
  docker run -dit --network exp1 --cap-add NET_ADMIN --ip $ip_address --mac-address $mac_address --name $container_name mempool sh -c "service ssh start && top"
  echo "已启动容器 $container_name，IP地址：$ip_address，MAC地址：$mac_address"

  # 将IP地址写入到 public_ips.txt 文件中
  echo $ip_address >> public_ips.txt

  container_index=$((container_index+1))
done
