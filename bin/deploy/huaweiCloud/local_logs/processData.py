import os
import json
import numpy as np
import matplotlib.pyplot as plt
from collections import defaultdict
from datetime import datetime

# Global variable f, you can set this as needed
f = 3

def read_json_files(folder_path):
    """读取指定文件夹中的所有 .json 文件"""
    files_data = []
    for file_name in os.listdir(folder_path):
        if file_name.endswith('.json'):
            with open(os.path.join(folder_path, file_name), 'r') as f:
                files_data.append(json.load(f))
    return files_data

def calculate_tps(stable_num_list, committed_num_list):
    """计算最大 TPS（斜率）"""
    tps = []
    for i in range(1, len(stable_num_list)):
        stable_diff = stable_num_list[i] - stable_num_list[i-1]
        committed_diff = committed_num_list[i] - committed_num_list[i-1]
        if stable_diff > 0:  # 防止除以零
            tps.append(stable_diff / stable_num_list[i-1])
    return max(tps) if tps else 0

def plot_latency_distribution(latencies, title):
    """绘制时延分布图"""
    plt.hist(latencies, bins=50, color='blue', alpha=0.7)
    plt.title(title)
    plt.xlabel('Latency (ms)')
    plt.ylabel('Frequency')
    plt.show()

def process_stable_time(stable_time_dict):
    """计算平均时延"""
    latencies = [stable_time_dict.values()]  # 转换为毫秒
    avg_latency = np.mean(latencies)
    print(f"Average Stable Time: {avg_latency} ms")
    plot_latency_distribution(latencies, 'Stable Time Latency Distribution')
    return avg_latency

def process_committed_time(committed_time_dict):
    """计算平均时延"""
    latencies = [committed_time_dict.values()]  # 转换为毫秒
    avg_latency = np.mean(latencies)
    print(f"Average Committed Time: {avg_latency} ms")
    plot_latency_distribution(latencies, 'Committed Time Latency Distribution')
    return avg_latency

def collect_execute_time_and_mb_received_time(files_data):
    """收集所有文件中的 execute_time 和 mb_received_time 字段并计算平均时延"""
    execute_times = defaultdict(list)
    mb_received_times = defaultdict(list)
    
    for data in files_data:
        for k, v in data.get("execute_time", {}).items():
            execute_times[k].append(v)  # 转为毫秒
        for k, v in data.get("mb_received_time", {}).items():
            mb_received_times[k].append(v)  # 转为毫秒

    avg_execute_time = {k: np.mean(v) for k, v in execute_times.items()}
    avg_mb_received_time = {k: np.mean(v) for k, v in mb_received_times.items()}
    
    print("Execute Time Avg Latency:", avg_execute_time)
    print("MB Received Time Avg Latency:", avg_mb_received_time)
    
    return avg_execute_time, avg_mb_received_time

def find_matching_positions(files_data):
    """处理 executed_num_list 和 committed_num_list"""
    matching_positions = []
    
    # 假设我们已经从文件中获取到所有的 executed_num_list 和 committed_num_list
    for idx, data in enumerate(files_data):
        executed_num_list = data.get("executed_num_list", [])
        committed_num_list = data.get("committed_num_list", [])
        
        for i in range(len(executed_num_list)):
            if executed_num_list[i] == committed_num_list[i]:
                matching_positions.append((i, executed_num_list[i]))
                
    return matching_positions



def convert_to_ms(time_str):
    """将时间字符串转换为毫秒"""
    try:
        if 'ms' in time_str:
            return float(time_str.replace('ms', ''))
        elif 's' in time_str:
            return float(time_str.replace('s', '')) * 1000
        elif 'us' in time_str:
            return float(time_str.replace('us', '')) / 1000
    except ValueError:
        return 0  # 无法转换时返回 0

def main():
    # 设置读取文件的文件夹路径
    folder_path = './logs'
    files_data = read_json_files(folder_path)
    
    # 处理 stable_num_list 和 committed_num_list，计算 TPS
    for data in files_data:
        stable_num_list = data.get("stable_num_list", [])
        committed_num_list = data.get("committed_num_list", [])
        max_tps = calculate_tps(stable_num_list, committed_num_list)
        print(f"Max TPS: {max_tps}")
    
    # 处理 stable_time 和 committed_time
    for data in files_data:
        stable_time = data.get("stable_time", {})
        committed_time = data.get("committed_time", {})
        process_stable_time(stable_time)
        process_committed_time(committed_time)
    
    # 收集 execute_time 和 mb_received_time
    collect_execute_time_and_mb_received_time(files_data)


if __name__ == "__main__":
    main()
