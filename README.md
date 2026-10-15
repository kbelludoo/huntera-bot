# ⚔ Huntera Multi-Bot Native Go Engine

Solução nativa desenvolvida em **Go (Golang)** de ultra-alta performance para o jogo **Huntera** (`huntera.com.br`), operando 100% em modo headless (WebSocket nativo / sem navegador), otimizada para VPS de baixo custo (Oracle Cloud Free Tier, AWS, etc.).

---

## ⚡ Vantagens da Arquitetura Go Nativa

| Recurso | Huntera Native Go Engine |
| :--- | :--- |
| **Uso de RAM** | **5 MB ~ 15 MB** (Ultra-leve) |
| **Uso de CPU** | **0.1% ~ 0.5%** |
| **Runtime** | **Binário Único Estático em Go (Zero Runtime)** |
| **Protocolo** | **WebSocket + Cipher RC4 / Protocolo Binário Nativo** |
| **Conexão** | **Multi-Contas Concorrentes (Goroutines)** |
| **Dashboard** | **HTTP Mux Nativo Go embutido (Porta 3000)** |

---

## 📁 Estrutura do Projeto

```
huntera-bot/
├── main.go                     # Ponto de entrada do Multi-Bot
├── go.mod                      # Módulo Go
├── go.sum                      # Checksums de dependências
├── pkg/
│   ├── bot/                    # Lógica de caça, stamina e bestiário
│   ├── client/                 # Cliente WebSocket e gerenciamento de sessão
│   ├── protocol/               # Cifras, opcodes e serialização de pacotes
│   └── server/                 # Servidor HTTP nativo /api/status (porta 3000)
├── docs/
│   ├── index.html              # Dashboard Web para GitHub Pages
│   └── status.json             # Telemetria anônima em tempo real
├── scripts/
│   ├── build-linux.bat         # Script para cross-compilar binário Linux
│   ├── start.sh                # Script para iniciar via tmux na VPS
│   └── bot.service             # Definição do serviço systemd
├── accounts.example.json       # Modelo de configuração de contas
└── .gitignore                  # Proteção de credenciais e binários
```

---

## 🚀 Como Compilar e Executar

### 1. Compilação Local (Windows)
```bash
go build -o huntera-bot.exe .
```

### 2. Compilar para Linux (VPS AMD64)
```bash
# Via script:
scripts/build-linux.bat

# Ou diretamente no terminal:
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -o huntera-bot-linux .
```

### 3. Configuração de Contas
Copie `accounts.example.json` para `accounts.json` e configure suas contas:
```json
[
  {
    "id": "acc1",
    "name": "Conta 1",
    "email": "seu-email@dominio.com",
    "password": "sua-senha",
    "characterName": "SeuChar",
    "huntId": "dark-cathedral-hunt",
    "tier": 0,
    "autoStamina": true,
    "targetStage": 7
  }
]
```

> **Aviso de Segurança:** O arquivo `accounts.json` é estritamente ignorado pelo Git (`.gitignore`). Nunca comite senhas ou credenciais no repositório.

---

## 📡 API de Telemetria e Dashboard
* **Dashboard Local:** `http://localhost:3000`
* **Endpoint de Status:** `http://localhost:3000/api/status`
* **GitHub Pages Hub:** Integrado automaticamente ao `bot-template-go` através do `vps-agent`.
