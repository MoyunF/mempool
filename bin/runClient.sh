#!/usr/bin/env bash

go build ../client/
# int=1
# while [ $int -lt  ]
# do
# ./client&
# let "int++"
# done
./client&
./client&
./client&
./client&
echo "1 clients are started"
