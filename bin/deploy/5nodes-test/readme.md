使用docker在本地跑五个节点进行测试

注:请使用linux环境或者wsl子系统

本地测试流程：
1.运行docker_run.sh 这会在本地启动5个docker，并且将日志文件挂载到当前目录的./local_logs下
2.运行send_to_dockers.sh 将编译后的程序和启动脚本发送到每个docker中
3.sh start_server.sh exp1 用来启动实验，接受一个参数作为本次实验的名称
4.sh stop_server.sh 停止实验 sh stop_server_clean.sh exp1 停止实验，并且清楚对应试验下的日志