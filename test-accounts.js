const { loginWithCredentials, getCharacters, getGameTicket } = require('./src/term/session');
const HunteraSocket = require('./src/term/huntera-socket');

const accounts = [
  { email: 'kbelludoo@gmail.com', password: 'ECTbb,,0209', defaultName: 'Maspopp' },
  { email: 'compraskbelludoo@gmail.com', password: 'ECTbb,,0209' },
  { email: 'brunofonsecadesouza89@gmail.com', password: 'ECTbb,,0209', defaultName: 'Kbelludook' },
  { email: 'brunofonsecadesouza1989@gmail.com', password: 'ECTbb,,0209' }
];

async function checkAccount(acc, index) {
  console.log(`\n=== [CONTA ${index + 1}] ${acc.email} ===`);
  try {
    const loginRes = await loginWithCredentials(acc.email, acc.password);
    console.log(`[OK] Login sucesso! ID: ${loginRes.account.id}`);

    const chars = await getCharacters(loginRes.cookie);
    console.log(`Personagens (${chars.length}):`, chars.map(c => `${c.name} (Lv. ${c.level})`).join(', '));

    const selectedChar = chars[0];
    if (!selectedChar) {
      console.log('Sem personagens nesta conta.');
      return;
    }

    acc.characterName = selectedChar.name;
    acc.characterId = selectedChar.id;
    acc.cookie = loginRes.cookie;

    const ticket = await getGameTicket(selectedChar.id, loginRes.cookie);
    console.log(`[OK] Ticket obtido para ${selectedChar.name}`);

    // Conectar WebSocket para ler player-stats e stamina
    await new Promise((resolve) => {
      const socket = new HunteraSocket({ cookie: loginRes.cookie });
      const timer = setTimeout(() => {
        socket.close();
        resolve();
      }, 5000);

      socket.on('player-stats', (msg) => {
        console.log(`[STATS] ${selectedChar.name}:`);
        console.log(`  HP: ${msg.health}/${msg.maxHealth} | Mana: ${msg.mana}/${msg.maxMana}`);
        console.log(`  StaminaMs: ${msg.staminaMs} | HuntSessionRemainingMs: ${msg.huntSessionRemainingMs}`);
        console.log(`  StaminaRefillCost: ${msg.staminaRefillCost} | RefillsLeft: ${msg.staminaRefillsLeft}`);
        clearTimeout(timer);
        socket.close();
        resolve();
      });

      socket.connect(selectedChar.name, ticket.ticket, ticket.websocketUrl);
    });

  } catch (e) {
    console.error(`[ERRO] ${acc.email}: ${e.message}`, e.data || '');
  }
}

async function run() {
  for (let i = 0; i < accounts.length; i++) {
    await checkAccount(accounts[i], i);
  }
}

run();
