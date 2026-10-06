#!/usr/bin/env bash
# Inicia as 4 contas do Huntera + Dashboard Web em background no tmux

tmux kill-session -t huntera 2>/dev/null
tmux kill-session -t huntera-1 2>/dev/null
tmux kill-session -t huntera-2 2>/dev/null
tmux kill-session -t huntera-3 2>/dev/null
tmux kill-session -t huntera-4 2>/dev/null
tmux kill-session -t huntera-web 2>/dev/null

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

echo "Iniciando Dashboard Web na porta 3000..."
tmux new-session -d -s huntera-web "cd $DIR && node web-server.js"

sleep 1

echo "Iniciando Conta 1 (Maspopp)..."
tmux new-session -d -s huntera-1 "cd $DIR && ACCOUNT=acc1 node src/term/main.js"

sleep 2

echo "Iniciando Conta 2 (Kbelludoos)..."
tmux new-session -d -s huntera-2 "cd $DIR && ACCOUNT=acc2 node src/term/main.js"

sleep 2

echo "Iniciando Conta 3 (Kbelludook)..."
tmux new-session -d -s huntera-3 "cd $DIR && ACCOUNT=acc3 node src/term/main.js"

sleep 2

echo "Iniciando Conta 4 (Kbelludoom)..."
tmux new-session -d -s huntera-4 "cd $DIR && ACCOUNT=acc4 node src/term/main.js"

echo "✓ Todas as 4 contas + Dashboard Web iniciados com sucesso!"
echo "Acesse o painel web em: http://129.153.157.17:3000"
