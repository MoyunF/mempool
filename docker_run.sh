# docker run -dit -p 8070:8070 --rm --network exp1 --ip 192.168.0.2 --mac-address 02:42:ac:11:00:02 --name container1 mempool
# docker run -dit -p 8071:8070 --rm --network exp1 --ip 192.168.0.3 --mac-address 02:42:ac:11:00:03 --name container2 mempool
# docker run -dit -p 8072:8070 --rm --network exp1 --ip 192.168.0.4 --mac-address 02:42:ac:11:00:04 --name container3 mempool
# docker run -dit -p 8073:8070 --rm --network exp1 --ip 192.168.0.5 --mac-address 02:42:ac:11:00:05 --name container4 mempool

docker run -dit --network exp1 --ip 192.168.0.2 --mac-address 02:42:ac:11:00:02 --name container1 mempool sh -c "service ssh start && top"
docker run -dit --network exp1 --ip 192.168.0.3 --mac-address 02:42:ac:11:00:03 --name container2 mempool sh -c "service ssh start && top"
docker run -dit --network exp1 --ip 192.168.0.4 --mac-address 02:42:ac:11:00:04 --name container3 mempool sh -c "service ssh start && top"
docker run -dit --network exp1 --ip 192.168.0.5 --mac-address 02:42:ac:11:00:05 --name container4 mempool sh -c "service ssh start && top"

docker run -dit --network exp1 --ip 192.168.0.6 --mac-address 02:42:ac:11:00:07 --name container5 mempool sh -c "service ssh start && top"
docker run -dit --network exp1 --ip 192.168.0.7 --mac-address 02:42:ac:11:00:08 --name container6 mempool sh -c "service ssh start && top"
docker run -dit --network exp1 --ip 192.168.0.8 --mac-address 02:42:ac:11:00:09 --name container7 mempool sh -c "service ssh start && top"
docker run -dit --network exp1 --ip 192.168.0.9 --mac-address 02:42:ac:11:00:10 --name container8 mempool sh -c "service ssh start && top"

docker run -dit --network exp1 --ip 192.168.0.10 --mac-address 02:42:ac:11:00:11 --name container9 mempool sh -c "service ssh start && top"
docker run -dit --network exp1 --ip 192.168.0.11 --mac-address 02:42:ac:11:00:12 --name container10 mempool sh -c "service ssh start && top"
