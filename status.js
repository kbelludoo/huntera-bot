/**
 * status.js
 * Monitor multi-contas que exibe um resumo ao vivo das 4 contas rodando na VPS.
 */

const { execSync } = require('child_process');

const accounts = ['huntera-1', 'huntera-2', 'huntera-3', 'huntera-4'];

console.log('\n================================================================================');
console.log('            ⚔  HUNTERA MULTI-BOT STATUS — 4 CONTAS SIMULTÂNEAS ⚔                ');
console.log('================================================================================\n');

for (const session of accounts) {
  try {
    const pane = execSync(`tmux capture-pane -pt ${session} -S -20 2>/dev/null`, { encoding: 'utf8' });
    const lines = pane.split('\n');

    let charLine = lines.find(l => l.includes('Personagem:')) || '';
    let huntLine = lines.find(l => l.includes('Hunt Atual:')) || '';
    let bestiaryLine = lines.find(l => l.includes('Bestiário :')) || '';
    let staminaLine = lines.find(l => l.includes('Stamina   :')) || '';

    // Limpar códigos ANSI para exibição limpa
    const clean = s => s.replace(/\x1b\[[0-9;]*m/g, '').trim();

    console.log(`[SESSÃO: ${session}]`);
    if (charLine) console.log(`  ${clean(charLine)}`);
    if (staminaLine) console.log(`  ${clean(staminaLine)}`);
    if (huntLine) console.log(`  ${clean(huntLine)}`);
    if (bestiaryLine) console.log(`  ${clean(bestiaryLine)}`);
    console.log('--------------------------------------------------------------------------------');
  } catch (e) {
    console.log(`[SESSÃO: ${session}] Offline ou não iniciada.`);
    console.log('--------------------------------------------------------------------------------');
  }
}

console.log('\nComandos úteis:');
console.log('  Ver tela ao vivo: tmux attach -t huntera-1 (Ctrl+B depois D para sair sem parar)');
console.log('  Parar todas:      tmux kill-server\n');
