#!/usr/bin/env bash
# 用来清理日志文件

LOG_DIR="."
LOG_FILES=$(find "$LOG_DIR" -type f -name "*.log")

if [ -n "$LOG_FILES" ]; then
    echo "Deleting log files..."
    find "$LOG_DIR" -type f -name "*.log" -exec rm -f {} \;
    echo "Log files deleted."
else
    echo "No log files found."
fi
