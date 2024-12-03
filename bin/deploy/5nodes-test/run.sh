#!/usr/bin/env bash
#用来在主机中启动collab主程序

SERVER_PID_FILE=server.pid

if [ -z "${SERVER_PID}" ]; then
    ./server -id $1 -log_dir=. -log_level=DEBUG -algorithm=hotstuff > program.log 2>&1 &
    echo $! >> ${SERVER_PID_FILE}
    echo "collab启动！"
else
    echo "Servers are already started in this folder."
    exit 0
fi
