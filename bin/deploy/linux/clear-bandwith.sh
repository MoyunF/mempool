#!/bin/bash

# 获取所有以"veth"开头的网卡名称
interfaces=$(ifconfig -a | grep -oP 'veth[a-z0-9]+' | sort)


# 计数器初始化为0
count=0

# 遍历每个网卡并限制带宽
for interface in $interfaces; do
  echo "清楚网卡 $interface 带宽限制"
  sudo sudo wondershaper -a $interface
  count=$((count+1))
done

echo "共清楚了 $count 个网卡的带宽。"
