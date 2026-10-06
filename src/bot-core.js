/**
 * bot-core.js
 * Motor headless do Huntera Bot v2.2.0.
 * Injetado no contexto da página. Não gera elementos visuais no DOM,
 * emitindo telemetria e logs diretamente para a interface do terminal no Node.js.
 */

(function () {
  'use strict';

  if (window.__huntera_loaded) return;
  window.__huntera_loaded = true;

  const BOT_VERSION = '2.2.0';
  const TICK_MS = 800;

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

  let cfg = Object.assign({
    autoHunt: true, smartHunt: false, huntId: 'rat-hunt', tier: 0, huntList: '',
    autoLoot: true, lootMinDelay: 350, lootMaxDelay: 900, disabledIds: '',
    autoRevive: true, reviveDelayMs: 3500, autoBless: false,
    autoExitRules: true, autoQuickSell: false,
    autoSustain: false, minHpPots: 20, minManaPots: 30, buyAmount: 100,
    autoStamina: false, idleFallback: false,
    sessionGuard: true, sessionWarnMins: 5, autoSessionLeave: false
  }, window.__huntera_config || {});

  const rand = (a, b) => a + Math.random() * (b - a);
  const now = () => Date.now();

  function emit(event, data) {
    if (typeof window.__huntera_emit === 'function') {
      try { window.__huntera_emit(event, data); } catch (_) {}
    }
  }

  function log(msg, kind) {
    emit('log', { msg, kind: kind || 'info', time: new Date().toLocaleTimeString('pt-BR') });
  }

  // ---------- hook Angular ----------
  let cached = { gs: null, sock: null };
  let hookStatus = 'procurando jogo…';

  function isGameState(o) {
    if (!o || (typeof o !== 'object' && typeof o !== 'function')) return false;
    try {
      return typeof o.takeLoot === 'function' && typeof o.startHunt === 'function'
        && typeof o.revive === 'function' && typeof o.loot === 'function'
        && typeof o.playerStats === 'function' && typeof o.huntPending === 'function';
    } catch (_) { return false; }
  }

  function isSocket(o) {
    if (!o || typeof o !== 'object') return false;
    try { return typeof o.send === 'function' && (typeof o.status === 'function' || typeof o.playerName === 'function' || 'serverUrl' in o); }
    catch (_) { return false; }
  }

  function bfsFind(root, pred, maxNodes) {
    const seen = new Set(); const q = [root];
    maxNodes = maxNodes || 4000; let n = 0;
    while (q.length && n < maxNodes) {
      const o = q.shift(); n++;
      if (!o || (typeof o !== 'object' && typeof o !== 'function')) continue;
      if (seen.has(o)) continue;
      seen.add(o);
      try { if (pred(o)) return o; } catch (_) {}
      if (o instanceof Node) continue;
      try {
        if (Array.isArray(o)) {
          for (let i = 0; i < o.length && q.length < maxNodes; i++) {
            const v = o[i];
            if (v && (typeof v === 'object' || typeof v === 'function') && !seen.has(v)) q.push(v);
          }
        } else {
          for (const k of Object.keys(o)) {
            if (k === '__proto__' || k === 'constructor') continue;
            let v; try { v = o[k]; } catch (_) { continue; }
            if (v && (typeof v === 'object' || typeof v === 'function') && !seen.has(v)) q.push(v);
          }
        }
      } catch (_) {}
    }
    return null;
  }

  // Prototype Trap autônomo (caso injetado diretamente)
  try {
    if (!window.__HUNTERA_PROTOTYPE_TRAP__) {
      window.__HUNTERA_PROTOTYPE_TRAP__ = true;
      Object.defineProperty(Object.prototype, 'gameState', {
        set(val) {
          if (val && typeof val === 'object' && typeof val.takeLoot === 'function') {
            window.__HUNTERA_GAME_STATE__ = val;
          }
          Object.defineProperty(this, 'gameState', {
            value: val,
            writable: true,
            configurable: true,
            enumerable: true
          });
        },
        configurable: true
      });

      Object.defineProperty(Object.prototype, 'socket', {
        set(val) {
          if (val && typeof val === 'object' && typeof val.send === 'function') {
            window.__HUNTERA_SOCKET__ = val;
          }
          Object.defineProperty(this, 'socket', {
            value: val,
            writable: true,
            configurable: true,
            enumerable: true
          });
        },
        configurable: true
      });
    }
  } catch (_) {}

  function findInstances() {
    if (cached.gs) {
      try { cached.gs.loot(); return cached; }
      catch (_) { cached = { gs: null, sock: null }; }
    }

    // 1. Hook via Prototype Trap (100% infalível no Angular 21 de produção)
    if (window.__HUNTERA_GAME_STATE__) {
      cached = {
        gs: window.__HUNTERA_GAME_STATE__,
        sock: window.__HUNTERA_SOCKET__ || window.__HUNTERA_GAME_STATE__.socket || null
      };
      hookStatus = 'hook Prototype Trap ✓';
      return cached;
    }

    try {
      const shellEl = document.querySelector('app-game-shell');
      if (shellEl && window.ng && typeof window.ng.getComponent === 'function') {
        const comp = window.ng.getComponent(shellEl);
        if (comp) {
          if (isGameState(comp.gameState)) {
            cached = { gs: comp.gameState, sock: comp.socket || comp.gameState.socket || null };
            hookStatus = 'hook ng.getComponent'; return cached;
          }
          const gs = bfsFind(comp, isGameState, 1500);
          if (gs) { cached = { gs, sock: comp.socket || gs.socket || null }; hookStatus = 'hook componente'; return cached; }
        }
      }
    } catch (e) { hookStatus = 'ng.getComponent: ' + (e && e.message); }

    try {
      const hosts = document.querySelectorAll('app-game-shell, app-root, app-hunt-window, app-loot-window, app-game-viewport');
      for (const el of hosts) {
        const ctx = el.__ngContext__;
        if (!ctx) continue;
        const gs = bfsFind(ctx, isGameState, 4000);
        if (gs) {
          const sock = (gs.socket && isSocket(gs.socket)) ? gs.socket : bfsFind(ctx, isSocket, 4000);
          cached = { gs, sock: sock || null }; hookStatus = 'hook __ngContext__'; return cached;
        }
      }
    } catch (e) { hookStatus = 'scan: ' + (e && e.message); }

    hookStatus = 'aguardando /game…';
    return cached;
  }

  function safeSig(fn, fb) {
    try { if (typeof fn !== 'function') return fb; const v = fn(); return v === undefined ? fb : v; }
    catch (_) { return fb; }
  }

  function callGs(name, ...args) {
    const gs = cached.gs;
    if (!gs || typeof gs[name] !== 'function') return false;
    try { gs[name](...args); return true; }
    catch (e) { log(`gameState.${name} falhou: ${e && e.message}`, 'err'); return false; }
  }

  // ---------- estado ----------
  const state = {
    charName: null, inGame: false, level: null, hp: null, maxHp: null, mana: null, maxMana: null, gold: 0,
    hunt: null, lootCount: 0, analyzer: null, sessionStart: now(), sessionRemainingMs: null,
    prey: null, premium: null, premiumEndsAt: null, disabledFeatures: [], chatPolicy: null,
    party: null, dps: null, sessionLeaveRequested: false, alerts: {},
    lootsTaken: 0, huntsStarted: 0, deaths: 0, buys: 0,
    catalog: [], paused: null, hookStatus: '',
    lastLootAt: 0, nextHuntAt: 0, reviveAt: 0, lastRuleSync: 0, lastStaminaBuy: 0,
    _deathAt: 0, _wasAlive: true, _partyWarned: false,
    shopSeq: null, lastShopAttempt: 0,
  };

  function detectPause() {
    try {
      const ch = document.querySelector('.world-challenge');
      if (ch && !ch.hidden && ch.offsetParent !== null) return 'desafio humano detectado (captcha)';
    } catch (_) {}
    try {
      if (cached.sock && typeof cached.sock.superseded === 'function' && cached.sock.superseded()) return 'sessão assumida em outra aba';
      if (cached.sock && typeof cached.sock.status === 'function' && cached.sock.status() === 'disconnected') return 'ws desconectado — aguardando reconexão';
    } catch (_) {}
    try {
      if (cached.gs && typeof cached.gs.loginQueue === 'function' && safeSig(cached.gs.loginQueue.bind(cached.gs), null)) return 'fila de login';
    } catch (_) {}
    return null;
  }

  function rawList() { return (cfg.huntList || '').split(',').map(s => s.trim()).filter(Boolean); }
  function huntListResolved() {
    const raw = rawList();
    if (cfg.smartHunt && raw.length) return raw;
    return [cfg.huntId];
  }

  function nextHuntInList(cur) {
    const list = huntListResolved();
    const i = list.indexOf(cur);
    if (i >= 0 && i + 1 < list.length) return list[i + 1];
    if (cfg.smartHunt && !rawList().length) {
      const j = HUNT_ORDER.indexOf(cur);
      if (j >= 0 && j + 1 < HUNT_ORDER.length) return HUNT_ORDER[j + 1];
    }
    return null;
  }

  function countPotions() {
    const gs = cached.gs;
    const inv = safeSig(gs.inventory ? gs.inventory.bind(gs) : null, null);
    let hp = 0, mana = 0;
    try {
      const all = [...(inv?.slots || []), ...(inv?.satchel || [])];
      for (const it of all) {
        if (!it || !it.name) continue;
        const nm = String(it.name).toLowerCase();
        const c = it.count || 1;
        if (nm.includes('health') || nm.includes('spirit')) hp += c;
        else if (nm.includes('mana')) mana += c;
      }
    } catch (_) {}
    return { hp, mana, gold: inv?.gold ?? 0 };
  }

  function syncRules() {
    const gs = cached.gs;
    if (!cfg.autoExitRules || now() - state.lastRuleSync < 30000) return;
    try {
      const cur = safeSig(gs.huntExitRules ? gs.huntExitRules.bind(gs) : null, null);
      if (cur && (cur.outOfGold !== true || cur.outOfCapacity !== true)) {
        if (callGs('setHuntExitRules', { outOfGold: true, outOfCapacity: true, partyMemberLeaves: false, goldFloor: null })) {
          log('exit-rules: sai sem gold/cap ✓', 'ok');
        }
      }
      const sell = safeSig(gs.huntSellRules ? gs.huntSellRules.bind(gs) : null, null);
      if (cfg.autoQuickSell && sell && (sell.onCapacityFull !== true)) {
        if (callGs('setHuntSellRules', { onCapacityFull: true, everyHalfHour: !!sell.everyHalfHour })) log('sell-rules: onCapacityFull ✓', 'ok');
      }
      state.lastRuleSync = now();
    } catch (_) {}
  }

  function syncLootFilter() {
    const gs = cached.gs;
    try {
      const ids = (cfg.disabledIds || '').split(',').map(s => parseInt(s.trim(), 10)).filter(n => Number.isInteger(n));
      const cur = safeSig(gs.autoLootDisabledItemIds ? gs.autoLootDisabledItemIds.bind(gs) : null, null);
      const curArr = cur ? [...cur].sort((a, b) => a - b) : [];
      if (JSON.stringify(curArr) !== JSON.stringify([...ids].sort((a, b) => a - b))) {
        if (callGs('setAutoLootDisabledItemIds', ids)) log(`filtro loot: ${ids.length} itens ignorados`);
      }
    } catch (_) {}
  }

  function sustainTick(ps, inCity, inHunt) {
    if (!cfg.autoSustain) return;
    const { hp, mana, gold } = countPotions();
    state.gold = gold;
    const needHp = Math.max(0, cfg.minHpPots - hp);
    const needMana = Math.max(0, cfg.minManaPots - mana);
    if ((needHp <= 0 && needMana <= 0) || state.shopSeq) return;
    if (!inCity || inHunt) return;
    if (now() - state.lastShopAttempt < 20000) return;
    state.lastShopAttempt = now();
    state.shopSeq = { step: 'wait-offers', since: now(), needHp, needMana };
    if (callGs('openShop')) log(`restock: abrindo shop (HP:${needHp} MP:${needMana})…`);
  }

  function shopTick() {
    const seq = state.shopSeq;
    if (!seq) return;
    const gs = cached.gs;
    if (now() - seq.since > 20000) {
      callGs('closeShop'); state.shopSeq = null; log('shop: timeout, fechado', 'warn'); return;
    }
    const shop = safeSig(gs.shop ? gs.shop.bind(gs) : null, null);
    if (!shop || !Array.isArray(shop.offers)) return;
    const offers = shop.offers;
    const find = (kind) => offers.filter(o => o.buyPrice != null && String(o.name || '').toLowerCase().includes(kind))
      .sort((a, b) => (a.buyPrice - b.buyPrice));
    const jobs = [];
    const hpOffers = find('health');
    const manaOffers = [...find('mana'), ...find('spirit')];
    if (seq.needHp > 0 && hpOffers.length) jobs.push({ offer: hpOffers[0], amount: Math.min(cfg.buyAmount, seq.needHp + cfg.buyAmount) });
    if (seq.needMana > 0 && manaOffers.length) jobs.push({ offer: manaOffers[0], amount: Math.min(cfg.buyAmount, seq.needMana + cfg.buyAmount) });
    for (const j of jobs) {
      if (callGs('buyShopItem', j.offer.itemId, j.amount)) {
        state.buys++;
        log(`comprou ${j.amount}x ${j.offer.name}`, 'ok');
      }
    }
    callGs('closeShop');
    state.shopSeq = null;
  }

  function staminaTick(ps) {
    if (!cfg.autoStamina || !ps) return;
    try {
      const ms = ps.staminaMs, cost = ps.staminaRefillCost, left = ps.staminaRefillsLeft;
      if (ms == null || cost == null || cost <= 0 || !(left > 0)) return;
      if (ms < 2 * 3600 * 1000 && now() - state.lastStaminaBuy > 10 * 60 * 1000) {
        const gold = safeSig(cached.gs.myGold ? cached.gs.myGold.bind(cached.gs) : null, 0);
        if (gold >= cost && callGs('buyStamina', cost)) {
          state.lastStaminaBuy = now();
          log(`stamina comprada por ${cost} gp`, 'ok');
        }
      }
    } catch (_) {}
  }

  function sessionGuardTick(inHunt) {
    const remaining = state.sessionRemainingMs;
    if (!cfg.sessionGuard || !inHunt || remaining === null || remaining === undefined) {
      if (!inHunt || remaining === null || remaining === undefined || remaining > 0) state.sessionLeaveRequested = false;
      return false;
    }
    const warnMs = Math.max(1, Number(cfg.sessionWarnMins) || 5) * 60 * 1000;
    if (remaining > 0 && remaining <= warnMs) {
      emit('alert', { type: 'session-warning', text: `sessão grátis termina em ${Math.round(remaining / 60000)}m` });
    }
    if (remaining <= 0) {
      if (!state.sessionLeaveRequested) {
        state.sessionLeaveRequested = true;
        if (cfg.autoSessionLeave && callGs('leaveHunt')) {
          log('sessão encerrada: saindo da hunt', 'warn');
        } else {
          log('sessão grátis encerrada — aguardando', 'warn');
        }
        state.nextHuntAt = now() + 30000;
      }
      return true;
    }
    return false;
  }

  function tick() {
    findInstances();
    const gs = cached.gs;
    state.hookStatus = hookStatus;

    if (!gs) {
      state.inGame = false;
      state.paused = 'fora do /game';
      emit('tick', state);
      return;
    }

    try {
      const sock = cached.sock || gs.socket;
      if (sock && typeof sock.playerName === 'function') {
        const nm = sock.playerName();
        if (nm && nm !== state.charName) {
          state.charName = nm;
          log(`personagem conectado: ${state.charName}`, 'ok');
        }
      }
    } catch (_) {}

    state.paused = detectPause();
    const ps = safeSig(gs.playerStats ? gs.playerStats.bind(gs) : null, null);
    state.inGame = !!ps;
    state.sessionRemainingMs = ps?.huntSessionRemainingMs ?? null;
    state.prey = safeSig(gs.prey ? gs.prey.bind(gs) : null, null);
    const premiumSignal = safeSig(gs.premium ? gs.premium.bind(gs) : null, null);
    state.premium = premiumSignal === null ? (state.prey ? !!state.prey.premium : null) : !!premiumSignal;
    state.premiumEndsAt = safeSig(gs.premiumEndsAt ? gs.premiumEndsAt.bind(gs) : null, state.prey?.premiumEndsAt ?? null);
    state.disabledFeatures = safeSig(gs.disabledFeatures ? gs.disabledFeatures.bind(gs) : null, []);
    state.chatPolicy = safeSig(gs.chatPolicy ? gs.chatPolicy.bind(gs) : null, null);
    state.party = safeSig(gs.party ? gs.party.bind(gs) : null, null);
    state.dps = safeSig(gs.dpsBreakdown ? gs.dpsBreakdown.bind(gs) : null, null);

    if (ps) {
      if (state.level !== null && ps.level > state.level) {
        log(`LEVEL UP ${state.level} → ${ps.level}!`, 'ok');
        emit('alert', { type: 'level-up', text: `Subiu para level ${ps.level}!` });
        if (cfg.smartHunt && state.hunt) {
          const nx = nextHuntInList(state.hunt.huntId);
          if (nx) {
            cfg.huntId = nx;
            log(`smart-hunt: avançando para ${nx}…`);
            callGs('changeHunt', nx, cfg.tier || 0);
          }
        }
      }
      state.level = ps.level; state.hp = ps.health; state.maxHp = ps.maxHealth;
      state.mana = ps.mana; state.maxMana = ps.maxMana;
    }

    const pending = safeSig(gs.huntPending ? gs.huntPending.bind(gs) : null, undefined);
    state.hunt = (pending && pending.hunt) ? pending.hunt : null;
    state.lootCount = (safeSig(gs.loot ? gs.loot.bind(gs) : null, []) || []).length;
    state.analyzer = safeSig(gs.huntAnalyzer ? gs.huntAnalyzer.bind(gs) : null, null);
    const death = safeSig(gs.death ? gs.death.bind(gs) : null, null);

    emit('tick', state);

    if (!state.inGame || state.paused) return;

    syncRules();
    if ((cfg.disabledIds || '').trim()) syncLootFilter();

    // Morte
    if (death) {
      if (state._wasAlive !== false) {
        state.deaths++;
        log(`morte detectada (killer: ${death.killer || '?'}). Aguardando revive…`, 'warn');
        emit('alert', { type: 'death', text: `Morreu para ${death.killer || 'monstro desconhecido'}` });
      }
      state._wasAlive = false;
      if (cfg.autoRevive && now() >= state.reviveAt) {
        if (!state._deathAt) state._deathAt = now();
        if (now() - state._deathAt >= ((death.reviveDelayMs || 0) + cfg.reviveDelayMs)) {
          if (callGs('revive')) {
            log('revive executado com sucesso', 'ok');
            state._deathAt = 0;
            state.reviveAt = now() + rand(2000, 4000);
            if (cfg.autoBless) setTimeout(() => {
              try {
                const b = safeSig(gs.blessings ? gs.blessings.bind(gs) : null, null);
                const gold = safeSig(gs.myGold ? gs.myGold.bind(gs) : null, 0);
                if (b && b.cost != null && (ps.level < b.freeUntilLevel || gold >= b.cost)) {
                  if (callGs('buyBlessing', 'all')) log('blessings compradas ✓', 'ok');
                }
              } catch (_) {}
            }, 4000);
          }
        }
      }
      return;
    }
    state._wasAlive = true; state._deathAt = 0;

    // Auto-Loot (1 item por tick com jitter humano)
    if (cfg.autoLoot && state.lootCount > 0 && now() - state.lastLootAt >= rand(cfg.lootMinDelay, cfg.lootMaxDelay)) {
      const items = safeSig(gs.loot.bind(gs), []);
      if (items && items.length && items[0]?.uid !== undefined) {
        if (callGs('takeLoot', items[0].uid)) {
          state.lootsTaken++;
          state.lastLootAt = now();
        }
      }
    }

    const leavePending = safeSig(gs.huntLeavePending ? gs.huntLeavePending.bind(gs) : null, null);
    const changePending = safeSig(gs.huntChangePending ? gs.huntChangePending.bind(gs) : null, false);
    const inHunt = safeSig(gs.inHunt ? gs.inHunt.bind(gs) : null, null);
    const inCity = safeSig(gs.inCity ? gs.inCity.bind(gs) : null, true);
    const inTown = safeSig(gs.inTown ? gs.inTown.bind(gs) : null, inCity);
    const sessionBlocked = sessionGuardTick(inHunt);

    // Auto-Hunt
    if (cfg.autoHunt && !sessionBlocked && !state.hunt && !leavePending && !changePending && now() >= state.nextHuntAt) {
      try {
        if (typeof gs.soloHuntLeavePartyConfirm === 'function' && gs.soloHuntLeavePartyConfirm()) {
          if (!state._partyWarned) { log('em party (não-líder): saia da party ou inicie manual', 'warn'); state._partyWarned = true; }
          state.nextHuntAt = now() + 10000;
          return;
        }
        state._partyWarned = false;
      } catch (_) {}

      if (inTown || inCity || inHunt === false) {
        if (callGs('startHunt', cfg.huntId, cfg.tier || 0)) {
          state.huntsStarted++;
          log(`iniciou hunt: ${cfg.huntId} (tier ${cfg.tier || 0})`, 'ok');
        }
        state.nextHuntAt = now() + rand(2500, 5000);
      } else if (cfg.idleFallback && inHunt === false) {
        try {
          if (typeof gs.setIdleTraining === 'function' && !safeSig(gs.idleTraining ? gs.idleTraining.bind(gs) : null, false)) {
            callGs('setIdleTraining', true);
            log('sem hunt: treino idle ativado');
          }
        } catch (_) {}
      }
    }

    // Quick-sell
    if (cfg.autoQuickSell && typeof gs.hasQuickSellItems === 'function') {
      try {
        if (gs.hasQuickSellItems()) {
          const cd = safeSig(gs.huntQuickSellCooldown ? gs.huntQuickSellCooldown.bind(gs) : null, null);
          if (!cd || !(cd.endsAt > now())) {
            if (callGs('confirmQuickSell')) log('quick-sell realizado ✓');
          }
        }
      } catch (_) {}
    }

    sustainTick(ps, inCity, inHunt);
    shopTick();
    staminaTick(ps);
  }

  log(`Huntera Bot v${BOT_VERSION} (Headless Core) carregado no jogo`, 'ok');
  setInterval(tick, TICK_MS);
})();
