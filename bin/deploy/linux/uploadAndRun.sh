sh exp_stop.sh
# sh clear.sh

#限制带宽
echo "清除带宽限制"
sudo sh clear-bandwith.sh
sh build.sh
sh deploy.sh
sh setup_cli.sh

#限制带宽
echo "限制带宽"
sudo bash ./limit-bandwith.sh 102400 102400 

sh start_server.sh
sh start_client.sh