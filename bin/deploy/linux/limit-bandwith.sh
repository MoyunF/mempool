#!/bin/bash

# 获取所有以"veth"开头的网卡名称
interfaces=$(ifconfig -a | grep -oP 'veth[a-z0-9]+' | sort)

# 设置带宽限制的值（以Kbps为单位）
bandwidth_limit="10240"

# 计数器初始化为0
count=0

# 遍历每个网卡并限制带宽
for interface in $interfaces; do
  echo "清楚网卡 $interface 带宽限制"
  sudo wondershaper $interface $1 $1 
  count=$((count+1))
done

echo "共限制了 $count 个网卡的带宽。"
