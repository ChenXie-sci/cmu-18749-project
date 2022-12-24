#! /bin/bash

bar="$1 $2 $3" 
echo $bar

cd ../servers
bash run.sh $1 $2 $3