sh exp_stop.sh
sh clear.sh

#限制带宽
echo "清除带宽限制"
sudo wondershaper -a veth0cb1654 
sudo wondershaper -a veth158b9e7 
sudo wondershaper -a veth295febe 
sudo wondershaper -a veth3c682d0 
sudo wondershaper -a veth9bfcaed
sudo wondershaper -a vethb873d03
sudo wondershaper -a vethb98dfb6
sudo wondershaper -a vethd691099
sudo wondershaper -a vetheeb940f
sudo wondershaper -a vethf33647f
sh build.sh
sh deploy.sh
sh setup_cli.sh

#限制带宽
echo "限制带宽"
sudo wondershaper veth0cb1654 10240 10240
sudo wondershaper veth158b9e7 10240 10240
sudo wondershaper veth295febe 10240 10240
sudo wondershaper veth3c682d0 10240 10240
sudo wondershaper veth9bfcaed 10240 10240
sudo wondershaper vethb873d03 10240 10240
sudo wondershaper vethb98dfb6 10240 10240
sudo wondershaper vethd691099 10240 10240
sudo wondershaper vetheeb940f 10240 10240
sudo wondershaper vetheeb940f 10240 10240
sudo wondershaper vethf33647f 10240 10240

sh start_server.sh
sh start_client.sh