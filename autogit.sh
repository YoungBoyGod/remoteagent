#!/bin/bash

for i in {1..100}
do 
    sleep 300
    git add . && git commit -m "add" && git push origin main
done
