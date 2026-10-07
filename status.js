/**
 * status.js
 * Monitor multi-contas que exibe um resumo ao vivo das 4 contas rodando na VPS.
 * Inclui: Ouro Total individual e acumulado, XP do nível, Bônus de XP do Bestiário e Kills Totais.
 */

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const dataDir = path.resolve(__dirname, 'data');
const accounts = [
  { id: 'acc1', session: 'huntera-1', label: 'Conta 1' },
  { id: 'acc2', session: 'huntera-2', label: 'Conta 2' },
  { id: 'acc3', session: 'huntera-3', label: 'Conta 3' },
  { id: 'acc4', session: 'huntera-4', label: 'Conta 4' }
];

function formatGp(n) {
  if (n === null || n === undefined || isNaN(n)) return '0 gp';
  const a = Math.abs(n);
  if (a < 1000) return `${Math.round(n)} gp`;
  if (a < 1e6) return `${(n / 1e3).toFixed(1)}k gp`;
  return `${(n / 1e6).toFixed(2)}kk gp`;
}

function formatStamina(ms) {
  if (!ms || ms <= 0) return '—';
  const totalMins = Math.floor(ms / 60000);
  const h = Math.floor(totalMins / 60);
  const m = totalMins % 60;
  return `${h}h ${String(m).padStart(2, '0')}m`;
}

console.log('\n================================================================================');
console.log('         ⚔  HUNTERA MULTI-BOT STATUS — 4 CONTAS SIMULTÂNEAS (VPS) ⚔             ');
console.log('================================================================================');

let totalGoldAll = 0;
let totalKillsAll = 0;
let activeCount = 0;

for (const acc of accounts) {
  const filePath = path.join(dataDir, `status_${acc.id}.json`);
  let data = null;

  if (fs.existsSync(filePath)) {
    try {
      data = JSON.parse(fs.readFileSync(filePath, 'utf8'));
    } catch (_) {}
  }

  if (data && data.charName) {
    activeCount++;
    const gold = Number(data.gold) || 0;
    totalGoldAll += gold;

    const b = data.bestiary || {};
    const bKills = b.kills || 0;
    const bReq = b.required || 2500;
    const bPct = bReq > 0 ? Math.min(100, Math.round((bKills / bReq) * 100)) : 0;
    const bBonus = b.bonusPercent || 0;
    const bComp = b.completedMonsters || 0;
    const bTotal = b.totalMonsters || 164;
    const totalCharKills = b.totalKills || (b.byMonster ? Object.values(b.byMonster).reduce((a, b) => a + (Number(b) || 0), 0) : 0);
    totalKillsAll += totalCharKills;

    const xp = Number(data.experience) || 0;
    const xpNeeded = Number(data.experienceNeeded) || 0;
    const xpPct = xpNeeded > 0 ? Math.round(xp * 100 / xpNeeded) : 0;
    const xpStr = xpNeeded > 0 ? `${xp.toLocaleString('pt-BR')} / ${xpNeeded.toLocaleString('pt-BR')} (${xpPct}%)` : (data.analyzer?.experience ? `+${data.analyzer.experience} xp (sessão)` : '—');

    const huntId = data.hunt ? `${data.hunt.huntId} (tier ${data.hunt.tier || 0})` : 'Aguardando';
    const stamina = formatStamina(data.staminaMs);
    const sessionRest = data.sessionRemainingMs ? formatStamina(data.sessionRemainingMs) : '—';

    console.log(`\n[${acc.label}: ${data.charName}] (Lv. ${data.level || '?'}) — Status: ${data.inGame ? '✅ EM JOGO' : '🔄 CONECTANDO'}`);
    console.log(`  💰 Ouro Mochila     : ${formatGp(gold)} (${gold.toLocaleString('pt-BR')} gp)`);
    console.log(`  📊 Progresso XP     : ${xpStr}`);
    console.log(`  ⭐ Bônus Bestiário  : +${bBonus}% XP Permanente | Concluídos: ${bComp}/${bTotal} monstros`);
    console.log(`  🎯 Monstro Atual    : ${b.currentMonster || '—'} [${bKills}/${bReq} kills (${bPct}%)]`);
    console.log(`  💀 Kills Totais     : ${totalCharKills.toLocaleString('pt-BR')} monstros abatidos`);
    console.log(`  🗡️ Hunt / Instância : ${huntId}`);
    console.log(`  ⏳ Stamina / Sessão : ${stamina} / ${sessionRest} restantes (Refills: ${data.staminaRefillsLeft ?? 6}/6)`);
    console.log(`  🌱 Modo             : 100% Zero-Waste (0 poções compradas / Farm contínuo)`);
    console.log('--------------------------------------------------------------------------------');
  } else {
    // Fallback: tentar ler via tmux pane
    try {
      const pane = execSync(`tmux capture-pane -pt ${acc.session} -S -20 2>/dev/null`, { encoding: 'utf8' });
      const lines = pane.split('\n');
      const clean = s => s.replace(/\x1b\[[0-9;]*m/g, '').trim();
      const charLine = lines.find(l => l.includes('Personagem:')) || '';
      const goldLine = lines.find(l => l.includes('Ouro Total:')) || '';
      const bestiaryLine = lines.find(l => l.includes('Bestiário :')) || '';
      const staminaLine = lines.find(l => l.includes('Stamina   :')) || '';

      console.log(`\n[${acc.label}: tmux ${acc.session}]`);
      if (charLine) console.log(`  ${clean(charLine)}`);
      if (goldLine) console.log(`  ${clean(goldLine)}`);
      if (bestiaryLine) console.log(`  ${clean(bestiaryLine)}`);
      if (staminaLine) console.log(`  ${clean(staminaLine)}`);
      console.log('--------------------------------------------------------------------------------');
    } catch (_) {
      console.log(`\n[${acc.label}: ${acc.session}] Offline ou não iniciada.`);
      console.log('--------------------------------------------------------------------------------');
    }
  }
}

console.log('\n================================================================================');
console.log('                        📈 RESUMO GLOBAL (TODAS AS CONTAS)                     ');
console.log('================================================================================');
console.log(`  💰 OURO TOTAL SOMADO   : ${formatGp(totalGoldAll)} (${totalGoldAll.toLocaleString('pt-BR')} gp)`);
console.log(`  💀 KILLS TOTAIS ACUM.  : ${totalKillsAll.toLocaleString('pt-BR')} monstros abatidos`);
console.log(`  👥 CONTAS ATIVAS       : ${activeCount} / 4 rodando simultaneamente`);
console.log('================================================================================\n');
console.log('Comandos úteis:');
console.log('  Ver tela ao vivo: tmux attach -t huntera-1 (Ctrl+B depois D para sair)');
console.log('  Reiniciar todas:  bash ~/huntera-cli/start-all.sh');
console.log('  Dashboard Web:    https://kbelludoo.github.io/huntera-bot/\n');
