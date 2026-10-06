/**
 * session.js
 * Gerenciador de sessão HTTP REST para o Huntera.
 * Obtém os tickets de entrada (/api/game-tickets) para alimentar a conexão WebSocket.
 */

const https = require('https');

const BASE_URL = 'https://huntera.com.br';

function httpRequest(path, options = {}, cookie = '') {
  return new Promise((resolve, reject) => {
    const url = new URL(path, BASE_URL);
    const headers = Object.assign({
      'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
      'Accept': 'application/json, text/plain, */*',
      'Content-Type': 'application/json'
    }, options.headers || {});

    if (cookie) {
      headers['Cookie'] = cookie;
    }

    const req = https.request(url, {
      method: options.method || 'GET',
      headers
    }, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          const json = data ? JSON.parse(data) : {};
          if (res.statusCode >= 200 && res.statusCode < 300) {
            resolve({ status: res.statusCode, data: json, headers: res.headers });
          } else {
            const err = new Error(json.message || `HTTP ${res.statusCode}`);
            err.status = res.statusCode;
            err.data = json;
            reject(err);
          }
        } catch (e) {
          reject(new Error(`Resposta inválida: ${data.slice(0, 100)}`));
        }
      });
    });

    req.on('error', reject);
    if (options.body) {
      req.write(typeof options.body === 'string' ? options.body : JSON.stringify(options.body));
    }
    req.end();
  });
}

/**
 * Carrega a conta autenticada
 */
async function loadMe(cookie) {
  const res = await httpRequest('/api/auth/me', { method: 'GET' }, cookie);
  return res.data?.account || null;
}

/**
 * Lista os personagens da conta
 */
async function getCharacters(cookie) {
  const res = await httpRequest('/api/characters', { method: 'GET' }, cookie);
  return Array.isArray(res.data) ? res.data : (res.data?.characters || []);
}

/**
 * Pede o ticket de jogo para entrar na sala do WebSocket (/game-socket)
 */
async function getGameTicket(characterId, cookie) {
  const res = await httpRequest('/api/game-tickets', {
    method: 'POST',
    body: { characterId }
  }, cookie);
  return res.data; // { ticket, websocketUrl, character }
}

/**
 * Realiza login com e-mail e senha diretamente no endpoint REST /api/auth/login
 * Retorna o cookie tw_session obtido nos headers
 */
async function loginWithCredentials(email, password) {
  const res = await httpRequest('/api/auth/login', {
    method: 'POST',
    body: { email, password }
  });

  const cookies = res.headers['set-cookie'] || [];
  let twSession = '';
  for (const c of cookies) {
    const match = c.match(/tw_session=([^;]+)/);
    if (match) {
      twSession = `tw_session=${match[1]}`;
      break;
    }
  }

  if (!twSession && cookies.length > 0) {
    twSession = cookies.map(c => c.split(';')[0]).join('; ');
  }

  return {
    account: res.data?.account,
    cookie: twSession
  };
}

module.exports = {
  loadMe,
  getCharacters,
  getGameTicket,
  loginWithCredentials,
  httpRequest
};
