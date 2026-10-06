/**
 * huntera-socket.js
 * Cliente WebSocket de alta performance para o Huntera.
 * Não usa navegador, consome zero CPU de renderização gráfica e ~35MB de RAM.
 */

const WebSocket = require('ws');
const EventEmitter = require('events');
const { encodePacket, decodeFrame, CLIENT_VERSION } = require('./protocol');

class HunteraSocket extends EventEmitter {
  constructor(options = {}) {
    super();
    this.options = options;
    this.ws = null;
    this.ticket = null;
    this.characterName = null;
    this.serverUrl = options.serverUrl || 'wss://huntera.com.br/game-socket';
    this.pingTimer = null;
    this.connected = false;
    this.latencyMs = 0;
  }

  connect(characterName, ticket, serverUrl = null) {
    this.characterName = characterName;
    this.ticket = ticket;
    if (serverUrl) this.serverUrl = serverUrl;

    if (this.ws) {
      try { this.ws.close(); } catch (_) {}
    }

    const headers = {
      'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
      'Origin': 'https://huntera.com.br'
    };

    if (this.options.cookie) {
      headers['Cookie'] = this.options.cookie;
    }

    this.ws = new WebSocket(this.serverUrl, { headers });
    this.ws.binaryType = 'arraybuffer';

    this.ws.on('open', () => {
      this.connected = true;
      this.emit('connected');

      // 1. Enviar autenticação
      this.send('authenticate', {
        clientVersion: CLIENT_VERSION,
        ticket: this.ticket
      });

      // 2. Loop de ping a cada 5 segundos
      if (this.pingTimer) clearInterval(this.pingTimer);
      this.pingTimer = setInterval(() => {
        if (this.connected) {
          this.send('ping', { t: Date.now() });
        }
      }, 5000);
    });

    this.ws.on('message', (data) => {
      const messages = decodeFrame(data);
      for (const msg of messages) {
        if (msg.type === 'pong') {
          this.latencyMs = Math.max(0, Date.now() - (msg.t || Date.now()));
          this.emit('latency', this.latencyMs);
          continue;
        }
        this.emit('message', msg);
        this.emit(msg.type, msg);
      }
    });

    this.ws.on('close', (code, reason) => {
      this.connected = false;
      if (this.pingTimer) clearInterval(this.pingTimer);
      this.emit('disconnected', { code, reason: reason ? reason.toString() : '' });
    });

    this.ws.on('error', (err) => {
      this.emit('error', err);
    });
  }

  send(type, payload = {}) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return false;
    try {
      const bytes = encodePacket(type, payload);
      this.ws.send(bytes);
      return true;
    } catch (e) {
      this.emit('error', new Error(`Falha ao enviar pacote ${type}: ${e.message}`));
      return false;
    }
  }

  // ---------- Ações de Jogo ----------
  startHunt(huntId, tier = 0) {
    return this.send('start-hunt', { huntId, tier });
  }

  changeHunt(huntId, tier = 0) {
    return this.send('change-hunt', { huntId, tier });
  }

  leaveHunt() {
    return this.send('leave-hunt', {});
  }

  takeLoot(uid) {
    return this.send('loot-take', { uid });
  }

  revive() {
    return this.send('revive', {});
  }

  buyBlessing(kind = 'all') {
    return this.send('blessing-buy', { kind });
  }

  openShop() {
    return this.send('shop-open', {});
  }

  buyShop(itemId, count = 100) {
    return this.send('shop-buy', { itemId, count });
  }

  buyStamina(cost) {
    return this.send('buy-stamina', { cost });
  }

  confirmQuickSell() {
    return this.send('hunt-quick-sell', {});
  }

  unlockBestiary(monsterId) {
    return this.send('bestiary-unlock', { monsterId });
  }

  requestCyclopedia() {
    return this.send('cyclopedia-request', {});
  }

  setIdleTraining(enabled = true) {
    return this.send('set-idle-training', { enabled });
  }

  close() {
    if (this.pingTimer) clearInterval(this.pingTimer);
    if (this.ws) {
      try { this.ws.close(); } catch (_) {}
      this.ws = null;
    }
    this.connected = false;
  }
}

module.exports = HunteraSocket;
