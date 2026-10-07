/**
 * main.js (Pure Terminal Driver — Sem Navegador)
 * Conexão direta via WebSocket e REST API.
 * Consome apenas ~35MB de RAM e ~0.2% de CPU.
 * 
 * Foco Principal:
 * 1. Fazer todos os bestiários sequencialmente (aumento de XP permanente).
 * 2. 100% Zero-Waste (zero poções compradas, 100% de lucro líquido).
 */

const fs = require('fs');
const path = require('path');
const TerminalUI = require('../terminal-ui');
const HunteraSocket = require('./huntera-socket');
const { loadMe, getCharacters, getGameTicket, loginWithCredentials } = require('./session');

// Ordem completa de progressão de Bestiário
const HUNT_ORDER = [
  "rat-hunt","spider-hunt","troll-hunt","swamp-troll-hunt","orc-hunt","folda-hunt",
  "skeleton-hunt","rotworm-hunt","dwarf-hunt","minotaur-hunt","gloom-ghost-wolf-hunt",
  "amazon-hunt","dark-cathedral-hunt","ghoul-hunt","rorc-hunt","yalahar-elf-hunt",
  "tarantula-hunt","scarab-hunt","swampling-hunt","tortoise-hunt","mutated-human-hunt",
  "cyclops-hunt","mummy-hunt","bonelord-hunt","orc-fortress-hunt","green-djinn-hunt",
  "blue-djinn-hunt","carlin-corym-hunt","cult-hunt","elder-forest-hunt","drefia-hunt",
  "ab-bonelord-hunt","ice-golem-hunt","lizard-steppe-hunt","brimstone-cave-hunt",
  "dragon-hunt","vampire-hunt","mutated-cave-hunt","bog-raider-hunt","giant-spider-hunt",
  "deeplings-hunt","hero-hunt","wyrm-hunt","zao-stronghold-hunt","grimvale-warrens-hunt",
  "rathleton-minotaurs-hunt","grimvale-dens-hunt","dragon-lord-hunt","war-golem-hunt",
  "lizard-chosen-hunt","werehyaena-hunt","werelion-hunt","behemoth-hunt","hellspawn-hunt",
  "hydra-hunt","seacrest-serpent-hunt","draken-walls-hunt","ripper-spectre-hunt",
  "goroma-serpent-spawn-hunt","asura-hunt","gazer-spectre-hunt","roshamuul-lower-hunt",
  "burster-spectre-hunt","grim-reaper-hunt","demon-hunt","falcon-hunt",
  "falcon-bastion-hunt","cobra-bastion-hunt","hell-hub-hunt","catacombs-hunt",
  "issavi-hunt","issavi-south-hunt","ice-library-hunt"
];

// Carregar configurações
const configPath = path.resolve(__dirname, '../../config.json');
let config = {};
try {
  if (fs.existsSync(configPath)) {
    config = JSON.parse(fs.readFileSync(configPath, 'utf8'));
  }
} catch (_) {}

// Suporte a multi-contas via accounts.json
const accountsPath = path.resolve(__dirname, '../../accounts.json');
let accounts = [];
try {
  if (fs.existsSync(accountsPath)) {
    accounts = JSON.parse(fs.readFileSync(accountsPath, 'utf8'));
  }
} catch (_) {}

if (process.env.ACCOUNT !== undefined) {
  const accParam = process.env.ACCOUNT;
  const targetAcc = accounts.find(a => a.id === accParam || String(a.index) === accParam) || accounts[Number(accParam)];
  if (targetAcc) {
    config = Object.assign({}, config, targetAcc);
  }
}

// Carregar catálogo de hunts salvo (se existir)
let huntCatalog = null;
const catalogPath = path.resolve(__dirname, '../../dump-hunt-catalog.json');
try {
  if (fs.existsSync(catalogPath)) {
    huntCatalog = JSON.parse(fs.readFileSync(catalogPath, 'utf8'));
  }
} catch (_) {}

// Carregar sessão (cookies)
const sessionPath = path.resolve(__dirname, '../../session.json');
let cookie = '';
if (fs.existsSync(sessionPath)) {
  try {
    const s = JSON.parse(fs.readFileSync(sessionPath, 'utf8'));
    if (typeof s.cookies === 'string') {
      cookie = s.cookies;
    } else if (Array.isArray(s.cookies)) {
      cookie = s.cookies.map(c => `${c.name}=${c.value}`).join('; ');
    }
  } catch (_) {}
}

const ui = new TerminalUI(config);

let currentHuntIndex = 0;
if (config.huntId) {
  const idx = HUNT_ORDER.indexOf(config.huntId);
  if (idx !== -1) currentHuntIndex = idx;
}

const accountId = config.id || process.env.ACCOUNT || 'acc1';
const statusDir = path.resolve(__dirname, '../../data');
if (!fs.existsSync(statusDir)) {
  try { fs.mkdirSync(statusDir, { recursive: true }); } catch (_) {}
}
const statusFile = path.join(statusDir, `status_${accountId}.json`);

const state = {
  charName: null,
  inGame: false,
  level: null,
  hp: null,
  maxHp: null,
  mana: null,
  maxMana: null,
  gold: 0,
  hunt: null,
  retreating: false,
  lootCount: 0,
  analyzer: { kills: 0, experience: 0, lootValue: 0, waste: 0 },
  myPlayerId: null,
  itemValues: {},
  lootsTaken: 0,
  huntsStarted: 0,
  deaths: 0,
  sessionRemainingMs: null,
  premium: false,
  paused: null,
  bestiary: {
    currentMonster: 'Iniciando…',
    monsterId: 'rat',
    kills: 0,
    required: 2500,
    bonusPercent: 0,
    completedMonsters: 0,
    totalMonsters: 164,
    stages: {},
    byMonster: {}
  }
};

function persistStatus() {
  try {
    const payload = {
      id: accountId,
      charName: state.charName,
      level: state.level,
      inGame: state.inGame,
      hp: state.hp,
      maxHp: state.maxHp,
      mana: state.mana,
      maxMana: state.maxMana,
      gold: state.gold,
      hunt: state.hunt,
      staminaMs: state.staminaMs,
      sessionRemainingMs: state.sessionRemainingMs,
      staminaRefillCost: state.staminaRefillCost,
      staminaRefillsLeft: state.staminaRefillsLeft,
      bestiary: state.bestiary,
      analyzer: state.analyzer,
      lootsTaken: state.lootsTaken,
      deaths: state.deaths,
      updatedAt: Date.now()
    };
    fs.writeFileSync(statusFile, JSON.stringify(payload, null, 2));
  } catch (_) {}
}

setInterval(persistStatus, 2000);

function advanceToNextHunt(socket) {
  if (currentHuntIndex + 1 < HUNT_ORDER.length) {
    currentHuntIndex++;
    const nextHunt = HUNT_ORDER[currentHuntIndex];
    config.huntId = nextHunt;
    ui.addLog(`🚀 [BESTIÁRIO] Avançando para próxima hunt: ${nextHunt} (${currentHuntIndex + 1}/${HUNT_ORDER.length})`, 'ok');
    socket.changeHunt(nextHunt, config.tier || 0);
  } else {
    ui.addLog(`🎉 TODOS OS BESTIÁRIOS CONCLUÍDOS COM SUCESSO!`, 'ok');
  }
}

async function start() {
  ui.addLog('Iniciando Huntera Driver Terminal Puro (Sem Navegador)…', 'info');

  // 1. Validar autenticação ou logar via e-mail e senha
  let me = null;

  if (config.email && config.password) {
    ui.addLog(`Autenticando via REST API (${config.email})…`, 'info');
    try {
      const loginRes = await loginWithCredentials(config.email, config.password);
      if (loginRes.cookie) {
        cookie = loginRes.cookie;
        ui.addLog(`Login automático efetuado com sucesso! (ID: ${loginRes.account?.id || '?'}) ✓`, 'ok');
      }
    } catch (e) {
      ui.addLog(`Falha no login com credenciais: ${e.message}`, 'warn');
    }
  }

  if (cookie) {
    try {
      me = await loadMe(cookie);
    } catch (_) {}
  }

  if (!me && config.email && config.password) {
    ui.addLog('Tentando re-autenticar via credenciais…', 'info');
    try {
      const loginRes = await loginWithCredentials(config.email, config.password);
      if (loginRes.cookie) {
        cookie = loginRes.cookie;
        me = await loadMe(cookie);
      }
    } catch (e) {
      ui.addLog(`Falha ao renovar autenticação: ${e.message}`, 'err');
    }
  }

  if (!me) {
    ui.addLog('Não foi possível autenticar. Forneça e-mail/senha ou cookie válido.', 'err');
    return;
  }

  ui.addLog(`Conta autenticada: ${me.email || me.id} ✓`, 'ok');

  // 2. Buscar personagens
  let chars = [];
  try {
    chars = await getCharacters(cookie);
    ui.addLog(`${chars.length} personagem(ns) encontrado(s)`, 'info');
  } catch (e) {
    ui.addLog(`Falha ao listar personagens: ${e.message}`, 'err');
    return;
  }

  if (chars.length === 0) {
    ui.addLog('Nenhum personagem disponível nesta conta.', 'warn');
    return;
  }

  // Selecionar personagem configurado ou o primeiro
  let targetChar = chars[0];
  if (config.characterName) {
    const found = chars.find(c => c.name.toLowerCase() === config.characterName.toLowerCase());
    if (found) targetChar = found;
  }

  ui.addLog(`Selecionando: ${targetChar.name} (Lv. ${targetChar.level || '?'})`, 'ok');
  state.charName = targetChar.name;
  state.level = targetChar.level;

  // 3. Obter Ticket do Jogo
  let ticketData;
  try {
    ticketData = await getGameTicket(targetChar.id, cookie);
    ui.addLog('Ticket de entrada obtido com sucesso ✓', 'ok');
  } catch (e) {
    ui.addLog(`Falha ao obter game ticket: ${e.message}`, 'err');
    return;
  }

  // 4. Conectar WebSocket
  const socket = new HunteraSocket({ cookie });

  socket.on('connected', () => {
    ui.addLog('Conectado ao WebSocket do Huntera (/game-socket) ✓', 'ok');
    state.inGame = true;
    ui.handleEvent('tick', state);

    setTimeout(() => {
      socket.requestCyclopedia();
    }, 1000);
  });

  socket.on('hunt-catalog', (msg) => {
    huntCatalog = msg;
  });

  // Identidade do próprio personagem (filtra eventos de outros jogadores)
  socket.on('welcome', (msg) => {
    if (msg.playerId) state.myPlayerId = msg.playerId;
  });

  // Catálogo de preços NPC: [[itemId, preco], ...] — usado para valorizar o loot
  socket.on('item-values', (msg) => {
    if (Array.isArray(msg.npc)) {
      for (const pair of msg.npc) {
        if (Array.isArray(pair) && pair.length >= 2) {
          state.itemValues[pair[0]] = pair[1];
        }
      }
    }
  });

  // Saldo real de gold da mochila
  socket.on('inventory-delta', (msg) => {
    if (typeof msg.gold === 'number') state.gold = msg.gold;
    ui.handleEvent('tick', state);
  });

  socket.on('player-stats', (msg) => {
    state.level = msg.level;
    state.hp = msg.health;
    state.maxHp = msg.maxHealth;
    state.mana = msg.mana;
    state.maxMana = msg.maxMana;
    state.sessionRemainingMs = msg.huntSessionRemainingMs ?? null;
    state.staminaMs = msg.staminaMs ?? null;
    state.staminaRefillCost = msg.staminaRefillCost ?? null;
    state.staminaRefillsLeft = msg.staminaRefillsLeft ?? 0;

    // Gerenciamento Inteligente de Stamina e Sessão de Caça
    if (state.sessionRemainingMs !== null && state.sessionRemainingMs > 0 && state.sessionRemainingMs <= 120000) {
      if (config.autoStamina && state.gold >= state.staminaRefillCost && state.staminaRefillsLeft > 0 && !state.buyingStamina) {
        state.buyingStamina = true;
        ui.addLog(`[STAMINA] Sessão baixa (< 2m). Recarregando por ${state.staminaRefillCost} gp…`, 'warn');
        socket.buyStamina(state.staminaRefillCost);
        setTimeout(() => { state.buyingStamina = false; }, 30000);
      } else if (!config.autoStamina && state.hunt && !state.sessionExpired) {
        state.sessionExpired = true;
        ui.addLog('[STAMINA] Sessão esgotada. Indo para Treino Idle gratuito (Zero-Waste)!', 'warn');
        socket.leaveHunt();
        socket.setIdleTraining(true);
      }
    }

    ui.handleEvent('tick', state);
  });

  socket.on('player-vitals', (msg) => {
    if (msg.health !== undefined) state.hp = msg.health;
    if (msg.mana !== undefined) state.mana = msg.mana;

    // Proteção Zero-Waste: Se o HP cair abaixo de 25%, recua da hunt para regenerar de graça
    if (state.hp && state.maxHp && (state.hp / state.maxHp < 0.25) && state.hunt && !state.retreating) {
      state.retreating = true;
      ui.addLog('HP < 25%: Recuando temporariamente para regeneração passiva (Zero-Waste: 0 poções)', 'warn');
      socket.leaveHunt();
      setTimeout(() => {
        state.retreating = false;
        ui.addLog('HP regenerado. Retomando caça de bestiário.', 'ok');
      }, 15000);
    }

    ui.handleEvent('tick', state);
  });

  // Confirmação de entrada na instância/hunt
  socket.on('instance-enter', (msg) => {
    const scId = msg.scenarioId;
    if (scId && scId !== 'main-city') {
      state.hunt = {
        huntId: scId,
        tier: msg.tier || 0,
        instanceId: msg.instanceId
      };
      ui.addLog(`Entrou na hunt: ${scId} (${msg.instanceId})`, 'ok');
    } else {
      state.hunt = null;
    }
    ui.handleEvent('tick', state);
  });

  socket.on('hunt-portal-entered', (msg) => {
    if (msg.huntId || msg.scenarioId) {
      state.hunt = { huntId: msg.huntId || msg.scenarioId, tier: msg.tier || 0 };
      ui.handleEvent('tick', state);
    }
  });

  socket.on('hunt-pending', (msg) => {
    if (msg.hunt) {
      state.hunt = msg.hunt;
      ui.handleEvent('tick', state);
    }
  });

  socket.on('hunt-leave-pending', () => {
    state.hunt = null;
    ui.addLog('Hunt finalizada/saindo...', 'info');
    ui.handleEvent('tick', state);
  });

  // Telemetria do analisador de caça
  socket.on('hunt-analyzer-update', (msg) => {
    state.analyzer = Object.assign(state.analyzer, msg);
    ui.handleEvent('tick', state);
  });

  socket.on('experience-gain', (msg) => {
    if (state.myPlayerId && msg.playerId && msg.playerId !== state.myPlayerId) return;
    const gain = Number(msg.value ?? msg.exp ?? 0);
    if (gain > 0) {
      state.analyzer.experience = (state.analyzer.experience || 0) + gain;
      state.analyzer.kills = (state.analyzer.kills || 0) + 1;
      ui.handleEvent('tick', state);
    }
  });

  // Monitoramento e Desbloqueio Automático de Bestiário
  socket.on('bestiary-progress', (msg) => {
    if (msg.bonusPercent !== undefined) state.bestiary.bonusPercent = msg.bonusPercent;
    if (msg.completed !== undefined) state.bestiary.completedMonsters = msg.completed;
    if (msg.total !== undefined) state.bestiary.totalMonsters = msg.total;

    // Identificar monstro atual da hunt
    const currentHuntId = state.hunt?.huntId || HUNT_ORDER[currentHuntIndex];
    const currentHuntObj = huntCatalog?.hunts?.find(h => h.id === currentHuntId);
    const targetMonster = currentHuntObj?.monsters?.[0];
    const bestiaryId = targetMonster?.bestiaryId || 'rat';
    const monsterName = targetMonster?.name || bestiaryId;

    if (msg.kills && typeof msg.kills === 'object') {
      state.bestiary.byMonster = Object.assign(state.bestiary.byMonster, msg.kills);
      state.bestiary.kills = msg.kills[bestiaryId] !== undefined ? msg.kills[bestiaryId] : (state.bestiary.kills || 0);
    }

    if (msg.stages && typeof msg.stages === 'object') {
      state.bestiary.stages = Object.assign(state.bestiary.stages, msg.stages);
    }

    state.bestiary.currentMonster = monsterName;
    state.bestiary.monsterId = bestiaryId;
    if (msg.killsRequired) state.bestiary.required = msg.killsRequired;

    // Desbloquear estágio automaticamente para obter bônus de XP
    const currentStages = state.bestiary.stages?.[bestiaryId] || 0;
    if (state.bestiary.kills >= state.bestiary.required && state.bestiary.required > 0) {
      socket.unlockBestiary(bestiaryId);
      ui.addLog(`⭐ [BESTIÁRIO] Desbloqueio enviado para ${monsterName} (+XP Permanente)`, 'ok');
    }

    // Se atingiu o teto do monstro dessa hunt, avançar automaticamente para a próxima
    if (currentStages >= 3 || (state.bestiary.kills >= 2500 && state.bestiary.required >= 2500)) {
      ui.addLog(`🏆 [BESTIÁRIO] Monstro ${monsterName} concluído com sucesso!`, 'ok');
      advanceToNextHunt(socket);
    }

    ui.handleEvent('tick', state);
  });

  // Coleta de Loot (Zero-Waste: 100% de Lucro)
  function handleLoot(msg) {
    const items = msg.items || (msg.item ? [msg.item] : (msg.uid ? [msg] : []));
    state.lootCount = items.length;

    // Valoriza o loot em tempo real (gold/h) via catálogo NPC
    let lootGain = 0;
    for (const it of items) {
      if (!it) continue;
      const count = Number(it.count) || 1;
      const unit = it.itemId === 3031 ? 1 : (state.itemValues[it.itemId] || 0);
      lootGain += unit * count;
      state.lootsTaken++;
    }
    if (lootGain > 0) {
      state.analyzer.lootValue = (state.analyzer.lootValue || 0) + lootGain;
    }
    ui.handleEvent('tick', state);

    if (config.autoLoot && items.length > 0 && !msg.item) {
      items.forEach((it, idx) => {
        if (it?.uid !== undefined) {
          setTimeout(() => {
            socket.takeLoot(it.uid);
            ui.addLog(`Loot coletado: ${it.name || it.uid} (Zero-Waste 100% Lucro)`, 'ok');
          }, (idx * 300) + Math.floor(Math.random() * 200 + 200));
        }
      });
    }
  }

  socket.on('loot', handleLoot);
  socket.on('loot-add', handleLoot);
  socket.on('loot-drop', handleLoot);

  socket.on('player-died', (msg) => {
    state.deaths++;
    state.hunt = null;
    ui.addLog(`Morte detectada! (${msg.killer || '?'})`, 'warn');
    if (config.autoRevive) {
      setTimeout(() => {
        socket.revive();
        ui.addLog('Revive executado ✓', 'ok');
      }, 3500);
    }
  });

  socket.on('disconnected', (info) => {
    state.inGame = false;
    state.hunt = null;
    ui.addLog(`WebSocket desconectado (código: ${info.code})`, 'warn');
    ui.handleEvent('tick', state);

    setTimeout(() => {
      ui.addLog('Tentando reconexão…', 'info');
      start().catch(() => {});
    }, 10000);
  });

  socket.on('error', (err) => {
    ui.addLog(`Erro de socket: ${err.message}`, 'err');
  });

  // Iniciar conexão com o servidor de jogo
  socket.connect(targetChar.name, ticketData.ticket, ticketData.websocketUrl);

  // Loop do bot: Auto-hunt focado em Bestiário Sequencial
  setInterval(() => {
    if (!state.inGame || state.hunt || state.retreating || !config.autoHunt) return;
    const targetHunt = HUNT_ORDER[currentHuntIndex] || config.huntId || 'rat-hunt';
    const tier = config.tier || 0;
    socket.startHunt(targetHunt, tier);
    state.huntsStarted++;
    ui.addLog(`Iniciando hunt focada em Bestiário: ${targetHunt} (tier ${tier})`, 'ok');
  }, 4000);
}

start().catch(err => {
  console.error('[Fatal Error]', err);
});
