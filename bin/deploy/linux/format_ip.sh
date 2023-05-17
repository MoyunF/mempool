#!/bin/bash

# 指定包含 IP 地址的文本文件路径
input_file="public_ips.txt"
# 指定输出文件路径
output_file="output.json"

# 关联数组存储 IP 地址
declare -A ip_addresses
declare -A http_addresses

# 读取文本文件中的 IP 地址
while IFS= read -r ip_address; do
  # 递增容器索引
  container_index=${#ip_addresses[@]}

  # 存储 IP 地址
  ip_addresses[$container_index+1]="tcp://$ip_address:3735"
  http_addresses[$container_index+1]="http://$ip_address:8070"
done < "$input_file"

# 输出 IP 地址到文件
{
  echo '"address": {'
  for index in "${!ip_addresses[@]}"; do
    echo "  \"$index\": \"${ip_addresses[$index]}\","
  done
  echo '},'

  echo '"http_address": {'
  for index in "${!http_addresses[@]}"; do
    echo "  \"$index\": \"${http_addresses[$index]}\","
  done
  echo '},'
} > "$output_file"
