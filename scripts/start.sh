#!/usr/bin/env bash
tmux kill-session -t huntera-go 2>/dev/null
sleep 1
tmux new-session -d -s huntera-go 'cd /home/ubuntu/huntera-bot && ./huntera-bot-linux'
