/**
 * web-server.js
 * Servidor HTTP leve para o Dashboard Web do Huntera Multi-Bot.
 * Roda na porta 3000 consumindo <15 MB de RAM.
 * Fornece API REST CORS (/api/status) para o GitHub Pages e serve a UI Web local.
 */

const http = require('http');
const fs = require('fs');
const path = require('path');

const PORT = process.env.PORT || 3000;
const DATA_DIR = path.resolve(__dirname, 'data');
const DOCS_DIR = path.resolve(__dirname, 'docs');

function getAccountsStatus() {
  const result = [];
  try {
    if (fs.existsSync(DATA_DIR)) {
      const files = fs.readdirSync(DATA_DIR).filter(f => f.startsWith('status_') && f.endsWith('.json'));
      for (const f of files.sort()) {
        try {
          const content = JSON.parse(fs.readFileSync(path.join(DATA_DIR, f), 'utf8'));
          result.push(content);
        } catch (_) {}
      }
    }
  } catch (_) {}
  return result;
}

const MIME_TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png'
};

const server = http.createServer((req, res) => {
  // CORS Headers para permitir requisições do GitHub Pages
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

  if (req.method === 'OPTIONS') {
    res.writeHead(204);
    res.end();
    return;
  }

  const url = new URL(req.url, `http://${req.headers.host || 'localhost'}`);

  // API endpoint de telemetria das 4 contas
  if (url.pathname === '/api/status') {
    const statuses = getAccountsStatus();
    res.writeHead(200, { 'Content-Type': 'application/json; charset=utf-8' });
    res.end(JSON.stringify({
      serverTime: Date.now(),
      accounts: statuses
    }));
    return;
  }

  // Servir Dashboard HTML estático
  let filePath = path.join(DOCS_DIR, url.pathname === '/' ? 'index.html' : url.pathname);
  if (!fs.existsSync(filePath) || fs.statSync(filePath).isDirectory()) {
    filePath = path.join(DOCS_DIR, 'index.html');
  }

  const ext = path.extname(filePath).toLowerCase();
  const contentType = MIME_TYPES[ext] || 'application/octet-stream';

  try {
    const content = fs.readFileSync(filePath);
    res.writeHead(200, { 'Content-Type': contentType });
    res.end(content);
  } catch (err) {
    res.writeHead(404, { 'Content-Type': 'text/plain' });
    res.end('404 Not Found');
  }
});

server.listen(PORT, '0.0.0.0', () => {
  console.log(`[Web Server] Dashboard rodando na porta ${PORT}: http://localhost:${PORT}`);
  console.log(`[Web Server] API pública: http://localhost:${PORT}/api/status`);
});
