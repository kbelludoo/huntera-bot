# ⚔ Huntera CLI — Headless Terminal Bot (1 vCPU / 1 GB RAM)

Solução desenvolvida para rodar o jogo **Huntera** (`huntera.com.br`) em **modo terminal 100% sem interface gráfica (headless)**, otimizada sob medida para instâncias gratuitas de VPS (ex: **Oracle Cloud Free Tier, AWS t2/t3.micro, Google Cloud e2-micro, Fly.io, etc.**) com apenas **1 vCPU e 1 GB de memória RAM**.

---

## ⚡ Como funciona a otimização de baixo consumo?

| Recurso | Navegador Comum (Com Gráficos) | Huntera CLI Headless (Sem Gráficos) |
| :--- | :--- | :--- |
| **Uso de RAM** | 800 MB ~ 1.4 GB *(Dá crash/OOM na VPS)* | **~140 MB a 220 MB** *(Estável e leve)* |
| **Uso de CPU** | 70% ~ 100% *(Renderizando sprites 60 FPS)* | **1% ~ 4%** *(Canvas e RAF throttled)* |
| **Consumo de Rede** | Download contínuo de texturas/áudio | **Apenas WebSockets e dados essenciais** |
| **Interface** | Janela gráfica com HTML/CSS pesado | **Dashboard ANSI no próprio Terminal (SSH)** |

### O que o otimizador faz:
1. **Bloqueio de Mídia na Rede:** Intercepta e descarta requisições de imagens (`.png`, `.jpg`, `.svg`), fontes (`.woff2`) e efeitos sonoros.
2. **Neutralização do Canvas/WebGL:** Desativa emuladores gráficos por software (`llvmpipe`), impedindo que o processador gaste ciclos desenhando frames invisíveis.
3. **Limitação de Taxa de Frames (RAF):** Reduz o loop de quadros para ~2 ticks/segundo (suficiente para o motor do jogo e troca de pacotes, economizando 95% de processamento).
4. **V8 Heap Cap:** Força o motor JavaScript a manter o heap abaixo de 256MB com coleta agressiva de lixo (`--max-old-space-size=256`).

---

## 🚀 Passo a Passo: Instalação na VPS (Ubuntu ou Debian)

### 1. Conectar na sua VPS via SSH
No seu terminal do PC:
```bash
ssh usuario@ip-da-sua-vps
```

### 2. Copiar os arquivos para a VPS
Você pode enviar a pasta `huntera-cli` para a sua VPS usando SCP ou Git:
```bash
# Exemplo via SCP (executado no seu PC):
scp -r ./huntera-cli usuario@ip-da-sua-vps:~/huntera-cli
```

### 3. Executar o Script de Instalação Automática
Acesse a pasta na VPS e execute o instalador (como root ou com `sudo`):
```bash
cd ~/huntera-cli
sudo bash setup-vps.sh
```

> **O que esse script faz:**
> - Cria automaticamente um arquivo de **SWAP de 2GB** (essencial para que a VPS de 1GB de RAM nunca trave por falta de memória).
> - Instala o **Node.js 20 LTS** e o **Chromium** com as bibliotecas mínimas de Linux.
> - Instala as dependências do projeto (`npm install`).

---

## 🔑 Autenticação: Conectando sua Conta sem Captcha

Para não precisar digitar login e senha nem enfrentar desafios de verificação no terminal:

1. No seu computador, abra o jogo no navegador e faça login no seu char: [https://huntera.com.br/game](https://huntera.com.br/game)
2. Pressione **F12** para abrir as Ferramentas de Desenvolvedor e vá na aba **Console**.
3. Abra o arquivo `export-session-helper.js` (ou copie o código abaixo) e cole no console do navegador:
   ```javascript
   copy(JSON.stringify({
     cookies: document.cookie,
     localStorage: Object.keys(localStorage).reduce((acc, k) => {
       acc[k] = localStorage.getItem(k); return acc;
     }, {})
   }, null, 2));
   ```
4. Pressione **Enter**. Os dados da sua sessão serão copiados para a sua área de transferência.
5. Na VPS, crie ou edite o arquivo `session.json`:
   ```bash
   nano session.json
   ```
6. Pressione `Ctrl+V` para colar e salve com `Ctrl+O`, `Enter` e saia com `Ctrl+X`.

Pronto! Ao iniciar o bot, ele entrará diretamente conectado na sua conta.

---

## 🎮 Como Rodar no Terminal

### Opção 1: Via `tmux` (Recomendado para acompanhar visualmente)
O `tmux` mantém o bot rodando mesmo quando você fecha a janela do SSH:

```bash
# 1. Cria uma nova sessão persistente:
tmux new -s huntera

# 2. Inicia o bot:
npm start
```

O dashboard em tempo real será exibido:
```text
╔════════════════════════════════════════════════════════════════════════════╗
║   ⚔  HUNTERA HEADLESS BOT — MODO TERMINAL (1 CPU / 1 GB RAM)               ║
╚════════════════════════════════════════════════════════════════════════════╝
  Personagem: Sir Knight (Lv. 65)      Status: [ EM JOGO ]
  HP        : [████████████████░░]  88% (940/1060)
  Mana      : [██████████████████] 100% (450/450)
  Gold      : 142.5k gp  │  Sessão: 02:40:15  │  RAM Node: 168 MB
  Hunt Atual: dragon-hunt (tier 0)  │  Loot Fila: 1
────────────────────────────────────────────────────────────────────────────
  [ ESTATÍSTICAS DA SESSÃO ]
  Tempo: 01:15:32  │  Kills: 132  │  XP/h: 210.5k/h
  Loots: 240 coletados  │  Mortes: 0  │  Saldo: +38.2k gp
────────────────────────────────────────────────────────────────────────────
  [ ATIVIDADE EM TEMPO REAL ]
  22:45:10 [ OK ] start-hunt dragon-hunt t0
  22:45:25 [ OK ] comprou 100x Health Potion
  22:45:40 [ OK ] auto-loot: Dragon Ham + Green Dragon Scale
────────────────────────────────────────────────────────────────────────────
```

- **Para sair do terminal sem desligar o bot:**
  Pressione `Ctrl + B` e em seguida aperte `D` (Detach).
- **Para voltar ao terminal do bot quando se conectar novamente:**
  ```bash
  tmux attach -t huntera
  ```

---

### Opção 2: Como Serviço do Sistema 24/7 (`systemd`)
Se você deseja que o bot inicie automaticamente se a VPS for reiniciada:

```bash
# 1. Copiar o arquivo de serviço para o sistema
sudo cp huntera.service /etc/systemd/system/

# 2. Ativar e iniciar o serviço
sudo systemctl daemon-reload
sudo systemctl enable --now huntera

# 3. Ver status e logs em tempo real:
sudo systemctl status huntera
sudo journalctl -u huntera -f
```

---

## ⚙ Personalização (`config.json`)

Edite o arquivo `config.json` para definir seu estilo de jogo:

```json
{
  "autoHunt": true,
  "smartHunt": false,
  "huntId": "dragon-hunt",
  "tier": 0,
  "huntList": "dragon-hunt, wyrm-hunt, hero-hunt",

  "autoLoot": true,
  "lootMinDelay": 350,
  "lootMaxDelay": 900,
  "disabledIds": "3035,3043",

  "autoRevive": true,
  "reviveDelayMs": 3500,
  "autoBless": true,

  "autoExitRules": true,
  "autoQuickSell": false,

  "autoSustain": true,
  "minHpPots": 30,
  "minManaPots": 40,
  "buyAmount": 100,

  "autoStamina": false,
  "idleFallback": true,

  "sessionGuard": true,
  "sessionWarnMins": 5,
  "autoSessionLeave": false,

  "webhookUrl": "https://discord.com/api/webhooks/...",
  "webhookMins": 30
}
```

### Principais Opções:
- `smartHunt`: Avança automaticamente para hunts de maior nível quando você sobe de level.
- `huntList`: Lista de hunts prioritárias personalizadas separadas por vírgula.
- `autoLoot`: Coleta itens com intervalo humanizado configurável (`lootMinDelay` e `lootMaxDelay`).
- `disabledIds`: IDs de itens para ignorar no loot (economiza espaço e cliques).
- `autoRevive`: Revive o personagem após a morte e compra blessings (`autoBless: true`).
- `autoSustain`: Abre a loja na cidade e compra poções quando ficarem abaixo do mínimo configurado.
- `webhookUrl`: Envia alertas de Level Up, mortes e relatórios de progresso direto no seu Discord.
