#! /bin/bash
bar="$1 $2 $3" 
xterm -ls bash run_server.sh "$1" "$2" "$3"