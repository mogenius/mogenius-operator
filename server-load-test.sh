#!/bin/bash
for ((i=1;i<=10;i++)); 
do 
   go run main.go testclient -d -c behrang.yaml -o "Cluster$i" &
done
wait

kill -9 $(pgrep -d' ' -f testclient)