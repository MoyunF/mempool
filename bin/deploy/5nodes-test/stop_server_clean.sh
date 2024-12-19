# 检查是否提供了参数
if [ -z "$1" ]; then
  echo "请输入要删除日志的实验，例如exp1"
  exit 1
fi

bash run_in_all.sh "bash ./kill_server.sh"
echo "进程全部终止"
bash run_in_all.sh "bash ./clean_logs.sh $1"
echo "日志全部清除"