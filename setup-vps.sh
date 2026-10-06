#!/usr/bin/env bash
# ==============================================================================
# setup-vps.sh — Instalador automático para VPS Free (1 vCPU / 1 GB RAM)
# Suporta: Ubuntu 20.04/22.04/24.04 e Debian 11/12
# ==============================================================================

set -e

echo "=========================================================="
echo "  ⚔  CONFIGURANDO AMBIENTE HUNTERA HEADLESS BOT (1GB RAM)  "
echo "=========================================================="

if [ "$EUID" -ne 0 ]; then
  echo "[-] Por favor, execute este script como root: sudo bash setup-vps.sh"
  exit 1
fi

# 1. Configurar SWAP de 2GB (CRÍTICO para VPS de 1GB de RAM não tomar OOM Crash)
echo "[+] 1/4 Verificando memória SWAP..."
SWAP_EXISTE=$(swapon --show | wc -l)
if [ "$SWAP_EXISTE" -le 1 ]; then
  echo "[+] Criando arquivo de SWAP de 2GB para estabilidade..."
  fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  if ! grep -q "/swapfile" /etc/fstab; then
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
  fi
  sysctl vm.swappiness=25
  echo 'vm.swappiness=25' >> /etc/sysctl.conf
  echo "[✓] SWAP de 2GB configurado com sucesso!"
else
  echo "[✓] SWAP já ativo no sistema."
fi

# 2. Atualizar pacotes do sistema e instalar dependências essenciais
echo "[+] 2/4 Atualizando repositórios e instalando bibliotecas para Chromium..."
apt-get update -y
apt-get install -y \
  curl \
  wget \
  tmux \
  ca-certificates \
  fonts-liberation \
  libatk-bridge2.0-0 \
  libatk1.0-0 \
  libcairo2 \
  libcups2 \
  libdbus-1-3 \
  libexpat1 \
  libfontconfig1 \
  libgbm1 \
  libgtk-3-0 \
  libnss3 \
  libpango-1.0-0 \
  libx11-6 \
  libxcomposite1 \
  libxdamage1 \
  libxfixes3 \
  libxrandr2 \
  libxss1 \
  xdg-utils

apt-get install -y libasound2t64 || apt-get install -y libasound2 || true

# 3. Instalar Node.js 20 LTS (se não estiver instalado)
echo "[+] 3/4 Verificando Node.js..."
if ! command -v node >/dev/null 2>&1; then
  echo "[+] Instalando Node.js 20 LTS..."
  curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
  apt-get install -y nodejs
fi
echo "[✓] Node.js $(node -v) e npm $(npm -v) prontos."

# 4. Instalar dependências do projeto npm
echo "[+] 4/4 Instalando dependências npm (Puppeteer)..."
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
cd "$DIR"
npm install --omit=dev
chown -R ubuntu:ubuntu "$DIR" 2>/dev/null || true

echo "=========================================================="
echo "  [✓] INSTALAÇÃO CONCLUÍDA COM SUCESSO!                   "
echo "=========================================================="
echo ""
echo "COMO USAR:"
echo "1. Crie ou cole sua sessão em: session.json (veja session.example.json)"
echo "2. Para iniciar no terminal dentro de uma sessão persistente tmux:"
echo "     tmux new -s huntera"
echo "     npm start"
echo "   (Para sair do tmux e deixar rodando: pressione Ctrl+B e depois D)"
echo "   (Para voltar ao terminal do bot a qualquer momento: tmux attach -t huntera)"
echo ""
echo "3. Para configurar como serviço 24/7 (systemd):"
echo "     cp huntera.service /etc/systemd/system/"
echo "     systemctl daemon-reload"
echo "     systemctl enable --now huntera"
echo "=========================================================="
