/**
 * terminal-ui.js
 * Painel TUI (Terminal User Interface) em tempo real via ANSI codes.
 * Leve, não consome memória extra e funciona perfeitamente via SSH (tmux / screen).
 */

const https = require('https');
const http = require('http');

// Códigos ANSI para cores
const C = {
  reset: '\x1b[0m',
  bold: '\x1b[1m',
  dim: '\x1b[2m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
  white: '\x1b[37m',
  gray: '\x1b[90m',
  bgDark: '\x1b[40m',
  clearScreen: '\x1b[2J\x1b[H'
};

class TerminalUI {
  constructor(config = {}) {
    this.config = config;
    this.logs = [];
    this.maxLogs = config.ui?.maxLogEntries || 10;
    this.lastState = null;
    this.startTime = Date.now();
    this.lastWebhook = 0;
    this.lastRender = 0;
    this.renderPending = false;
  }

  addLog(msg, kind = 'info', time = null) {
    const t = time || new Date().toLocaleTimeString('pt-BR');
    this.logs.unshift({ msg, kind, time: t });
    if (this.logs.length > this.maxLogs) this.logs.pop();
    this.requestRender();
  }

  handleEvent(event, data) {
    if (event === 'log') {
      this.addLog(data.msg, data.kind, data.time);
    } else if (event === 'tick') {
      this.lastState = data;
      this.requestRender();
    } else if (event === 'alert') {
      this.addLog(`⚠ ${data.text}`, 'warn');
      this.sendWebhook(`[Alerta] ${data.text}`);
    }
  }

  requestRender() {
    const now = Date.now();
    if (now - this.lastRender >= 200) {
      this.render();
    } else if (!this.renderPending) {
      this.renderPending = true;
      setTimeout(() => {
        this.renderPending = false;
        this.render();
      }, 200 - (now - this.lastRender));
    }
  }

  formatBar(val, max, width = 18, filledColor = C.green) {
    if (val == null || max == null || max <= 0 || isNaN(val) || isNaN(max)) {
      return `${C.gray}[${' '.repeat(width)}]  —%${C.reset}`;
    }
    const ratio = Math.max(0, Math.min(1, Number(val) / Number(max)));
    const filledChars = Math.round(ratio * width);
    const emptyChars = width - filledChars;
    const bar = `${filledColor}${'█'.repeat(filledChars)}${C.gray}${'░'.repeat(emptyChars)}${C.reset}`;
    const pct = Math.round(ratio * 100);
    return `[${bar}] ${String(pct).padStart(3)}% (${val}/${max})`;
  }

  formatGp(n) {
    if (n === null || n === undefined) return '—';
    const a = Math.abs(n);
    if (a < 1000) return `${Math.round(n)} gp`;
    if (a < 1e6) return `${(n / 1e3).toFixed(1)}k gp`;
    return `${(n / 1e6).toFixed(2)}kk gp`;
  }

  formatStamina(ms) {
    if (!ms || ms <= 0) return '—';
    const totalMins = Math.floor(ms / 60000);
    const h = Math.floor(totalMins / 60);
    const m = totalMins % 60;
    return `${h}h ${String(m).padStart(2, '0')}m`;
  }

  formatTime(ms) {
    if (!ms || ms < 0) return '00:00:00';
    const totalSecs = Math.floor(ms / 1000);
    const h = String(Math.floor(totalSecs / 3600)).padStart(2, '0');
    const m = String(Math.floor((totalSecs % 3600) / 60)).padStart(2, '0');
    const s = String(totalSecs % 60).padStart(2, '0');
    return `${h}:${m}:${s}`;
  }

  sendWebhook(text) {
    const url = this.config.webhookUrl;
    if (!url || !url.startsWith('http')) return;
    try {
      const char = this.lastState?.charName || 'HunteraBot';
      const body = JSON.stringify({ content: `[${char}] ${text}` });
      const parsed = new URL(url);
      const req = (parsed.protocol === 'https:' ? https : http).request(parsed, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(body) }
      });
      req.on('error', () => {});
      req.write(body);
      req.end();
    } catch (_) {}
  }

  render() {
    const now = Date.now();
    if (now - this.lastRender < 200) return; // Throttle renders
    this.lastRender = now;

    const s = this.lastState || {};
    const mem = process.memoryUsage();
    const memMb = Math.round(mem.rss / 1024 / 1024);

    const a = s.analyzer || {};
    const duration = now - this.startTime;
    const xpHour = duration > 30000 ? Math.round((Number(a.experience) || 0) * 3600000 / duration) : 0;
    const goldHour = duration > 30000 ? Math.round((Number(a.lootValue) || 0) * 3600000 / duration) : 0;
    const balance = (Number(a.lootValue) || 0) - (Number(a.waste) || 0);

    let statusBadge = `${C.gray}[ CONECTANDO ]${C.reset}`;
    if (s.paused) {
      statusBadge = `${C.yellow}${C.bold}[ PAUSADO: ${s.paused} ]${C.reset}`;
    } else if (s.inGame) {
      statusBadge = `${C.green}${C.bold}[ EM JOGO ]${C.reset}`;
    }

    const lines = [];
    lines.push(`${C.cyan}╔════════════════════════════════════════════════════════════════════════════╗${C.reset}`);
    lines.push(`${C.cyan}║   ${C.bold}⚔  HUNTERA HEADLESS BOT — MODO TERMINAL (1 CPU / 1 GB RAM)${C.reset}${C.cyan}               ║${C.reset}`);
    lines.push(`${C.cyan}╚════════════════════════════════════════════════════════════════════════════╝${C.reset}`);
    
    lines.push(`  ${C.bold}Personagem:${C.reset} ${C.white}${s.charName || 'Detectando…'}${C.reset} (Lv. ${s.level ?? '—'})      ${C.bold}Status:${C.reset} ${statusBadge}`);
    lines.push(`  ${C.bold}HP        :${C.reset} ${this.formatBar(s.hp, s.maxHp, 16, C.green)}`);
    lines.push(`  ${C.bold}Mana      :${C.reset} ${this.formatBar(s.mana, s.maxMana, 16, C.cyan)}`);
    lines.push(`  ${C.bold}Stamina   :${C.reset} ${this.formatStamina(s.staminaMs)}  │  ${C.bold}Sessão:${C.reset} ${s.sessionRemainingMs ? this.formatTime(s.sessionRemainingMs) : (s.premium ? 'Ilimitada (VIP)' : '—')}  │  ${C.bold}Refills:${C.reset} ${s.staminaRefillsLeft ?? '—'}/6  │  ${C.bold}RAM:${C.reset} ${memMb} MB`);
    lines.push(`  ${C.bold}Hunt Atual:${C.reset} ${s.hunt ? `${s.hunt.huntId} (tier ${s.hunt.tier || 0})` : 'Aguardando início'}  │  ${C.bold}Loot Fila:${C.reset} ${s.lootCount || 0}`);
    if (s.bestiary) {
      const b = s.bestiary;
      const bProgress = b.required ? `${b.kills || 0}/${b.required} (${Math.min(100, Math.round((b.kills || 0) * 100 / b.required))}%)` : `${b.kills || 0}`;
      lines.push(`  ${C.bold}Bestiário :${C.reset} ${C.magenta}${b.currentMonster || 'Em progresso'}${C.reset} [${bProgress}]  │  ${C.bold}Bônus XP:${C.reset} ${C.green}+${b.bonusPercent || 0}%${C.reset}  │  ${C.bold}Concluídos:${C.reset} ${b.completedMonsters || 0}`);
    }
    lines.push(`  ${C.bold}Estratégia:${C.reset} ${C.green}100% ZERO-WASTE${C.reset} (0 poções compradas / Farm de Bestiário sequencial)`);
    lines.push(`${C.gray}────────────────────────────────────────────────────────────────────────────${C.reset}`);
    lines.push(`  ${C.bold}[ ESTATÍSTICAS DA SESSÃO ]${C.reset}`);
    lines.push(`  Tempo: ${this.formatTime(duration)}  │  Kills: ${a.kills || 0}  │  XP/h: ${this.formatGp(xpHour)}/h  │  Gold/h: ${this.formatGp(goldHour)}/h`);
    lines.push(`  Loots: ${s.lootsTaken || 0} coletados  │  Mortes: ${s.deaths || 0}  │  Saldo: ${balance >= 0 ? C.green : C.red}${this.formatGp(balance)}${C.reset}`);
    lines.push(`${C.gray}────────────────────────────────────────────────────────────────────────────${C.reset}`);
    lines.push(`  ${C.bold}[ ATIVIDADE EM TEMPO REAL ]${C.reset}`);

    if (this.logs.length === 0) {
      lines.push(`  ${C.gray}Aguardando primeiras ações…${C.reset}`);
    } else {
      for (const entry of this.logs) {
        let color = C.white;
        let prefix = '[INFO]';
        if (entry.kind === 'ok') { color = C.green; prefix = '[ OK ]'; }
        else if (entry.kind === 'warn') { color = C.yellow; prefix = '[WARN]'; }
        else if (entry.kind === 'err') { color = C.red; prefix = '[ERRO]'; }

        lines.push(`  ${C.gray}${entry.time}${C.reset} ${color}${prefix}${C.reset} ${entry.msg}`);
      }
    }

    lines.push(`${C.cyan}────────────────────────────────────────────────────────────────────────────${C.reset}`);
    lines.push(`  ${C.dim}Ctrl+C para encerrar com segurança. Executando em background.${C.reset}`);

    process.stdout.write(C.clearScreen + lines.join('\n') + '\n');

    // Webhook periódico
    if (this.config.webhookUrl && now - this.lastWebhook > (this.config.webhookMins || 30) * 60000) {
      this.lastWebhook = now;
      this.sendWebhook(`📊 Status: Lvl ${s.level || '?'} · Hunt: ${s.hunt?.huntId || '—'} · Kills: ${a.kills || 0} · Saldo: ${this.formatGp(balance)} · Mortes: ${s.deaths || 0}`);
    }
  }
}

module.exports = TerminalUI;
