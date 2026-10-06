const fs = require('fs');
const path = require('path');
const HunteraSocket = require('./huntera-socket');
const { loadMe, getCharacters, getGameTicket } = require('./session');

const sessionPath = path.resolve(__dirname, '../../session.json');
let cookie = '';
if (fs.existsSync(sessionPath)) {
  const s = JSON.parse(fs.readFileSync(sessionPath, 'utf8'));
  cookie = typeof s.cookies === 'string' ? s.cookies : s.cookies.map(c => `${c.name}=${c.value}`).join('; ');
}

async function run() {
  console.log('[PROBE] Carregando me e chars...');
  const me = await loadMe(cookie);
  console.log('[PROBE] Me:', me.email || me.id);
  const chars = await getCharacters(cookie);
  const char = chars.find(c => c.name === 'Maspopp') || chars[0];
  console.log('[PROBE] Char:', char.name, 'Level:', char.level);

  const ticket = await getGameTicket(char.id, cookie);
  console.log('[PROBE] Ticket ok, conectando ws...');

  const socket = new HunteraSocket({ cookie });
  const messageCounts = {};

  socket.on('message', (msg) => {
    messageCounts[msg.type] = (messageCounts[msg.type] || 0) + 1;
    if (msg.type === 'bestiary-progress') {
      fs.writeFileSync('/home/ubuntu/huntera-cli/dump-bestiary.json', JSON.stringify(msg, null, 2));
    }
    if (msg.type === 'hunt-catalog') {
      fs.writeFileSync('/home/ubuntu/huntera-cli/dump-hunt-catalog.json', JSON.stringify(msg, null, 2));
    }
    if (['instance-enter', 'bestiary-progress', 'hunt-pending', 'loot-drop', 'loot-add', 'loot', 'experience-gain'].includes(msg.type)) {
      console.log(`[PROBE MSG] ${msg.type}:`, JSON.stringify(msg).slice(0, 150));
    }
  });

  socket.on('connected', () => {
    console.log('[PROBE] Conectado!');
    setTimeout(() => {
      console.log('[PROBE] Solicitando cyclopedia-request...');
      socket.send('cyclopedia-request', {});
    }, 1500);

    setTimeout(() => {
      console.log('[PROBE] Enviando start-hunt rat-hunt...');
      socket.startHunt('rat-hunt', 0);
    }, 3000);

    setTimeout(() => {
      console.log('[PROBE] Mensagens recebidas resumo:', messageCounts);
      process.exit(0);
    }, 15000);
  });

  socket.connect(char.name, ticket.ticket, ticket.websocketUrl);
}

run().catch(err => {
  console.error('[PROBE ERR]', err);
  process.exit(1);
});
