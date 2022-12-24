#!/bin/bash

bar="$1 $2 $3" 
echo $bar

osascript -ss -  "$1" "$2" "$3"  <<EOF
    on run argv -- argv is a list of strings
        set argList to {}
        repeat with arg in argv
            set end of argList to quoted form of arg
        end repeat
        set {TID, text item delimiters} to {text item delimiters, space}
        set argList to argList as text
        set text item delimiters to TID
        tell application "Terminal"
            activate
            # need to change path #
            do script ("cd /Users/mhkuo/Code/18749/Project/servers && ./run.sh " & argList & " 2>&1")
        end tell
    end run
EOF