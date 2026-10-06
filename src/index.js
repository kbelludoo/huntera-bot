/**
 * index.js
 * Ponto de entrada do Huntera Headless Bot.
 * Otimizado especificamente para rodar em VPS gratuita de 1 vCPU e 1 GB RAM.
 */

const fs = require('fs');
const path = require('path');
const puppeteer = require('puppeteer');
const TerminalUI = require('./terminal-ui');
const { applyPageOptimizations, CHROMIUM_OPTIMIZED_FLAGS } = require('./optimizer');

// Carregar configurações
const configPath = path.resolve(__dirname, '../config.json');
let config = {};
try {
  if (fs.existsSync(configPath)) {
    config = JSON.parse(fs.readFileSync(configPath, 'utf8'));
  }
} catch (e) {
  console.error('[Config] Erro ao carregar config.json:', e);
}

// Suporte a flags de linha de comando
const args = process.argv.slice(2);
const isGui = args.includes('--gui') || args.includes('--no-headless');
const isHeadless = !isGui && (config.headless !== false);

const ui = new TerminalUI(config);

// Carregar sessão salva (cookies / localStorage)
let sessionData = null;
const sessionPath = path.resolve(__dirname, '../session.json');
if (fs.existsSync(sessionPath)) {
  try {
    sessionData = JSON.parse(fs.readFileSync(sessionPath, 'utf8'));
    ui.addLog('Arquivo session.json carregado com sucesso', 'ok');
  } catch (e) {
    ui.addLog(`Erro ao ler session.json: ${e.message}`, 'warn');
  }
}

// Ler o código do bot-core.js
const botCoreCode = fs.readFileSync(path.resolve(__dirname, 'bot-core.js'), 'utf8');

async function launchBot() {
  ui.addLog('Inicializando Chromium ultra-leve (sem gráficos)…', 'info');

  const launchOptions = {
    headless: isHeadless ? 'new' : false,
    args: [...CHROMIUM_OPTIMIZED_FLAGS],
    userDataDir: path.resolve(__dirname, '../', config.userDataDir || './data/session_data'),
    defaultViewport: { width: 800, height: 600 }
  };

  // Se houver binário do sistema configurado ou detectado
  if (config.executablePath && fs.existsSync(config.executablePath)) {
    launchOptions.executablePath = config.executablePath;
  } else if (process.platform === 'linux') {
    const commonLinuxPaths = [
      '/usr/bin/chromium',
      '/usr/bin/chromium-browser',
      '/usr/bin/google-chrome-stable'
    ];
    for (const p of commonLinuxPaths) {
      if (fs.existsSync(p)) {
        launchOptions.executablePath = p;
        break;
      }
    }
  }

  let browser;
  try {
    browser = await puppeteer.launch(launchOptions);
  } catch (err) {
    ui.addLog(`Falha ao iniciar Chromium: ${err.message}`, 'err');
    if (process.platform === 'linux') {
      console.log('\n[DICA] No Ubuntu/Debian, instale as dependências com:');
      console.log('sudo apt-get install -y chromium-browser libnss3 libatk1.0-0 libatk-bridge2.0-0 libcups2 libdrm2 libxkbcommon0 libxcomposite1 libxdamage1 libxfixes3 libxrandr2 libgbm1 libasound2\n');
    }
    process.exit(1);
  }

  const pages = await browser.pages();
  const page = pages.length > 0 ? pages[0] : await browser.newPage();

  // Expor função de comunicação para o bot dentro do navegador emitir para o terminal Node
  await page.exposeFunction('__huntera_emit', (event, data) => {
    ui.handleEvent(event, data);
  });

  // Injetar configurações antes de carregar os scripts
  await page.evaluateOnNewDocument((cfg) => {
    window.__huntera_config = cfg;
  }, config);

  // Injetar otimizador (bloqueio de imagens, neutralizador de Canvas/WebGL, etc.)
  await applyPageOptimizations(page, sessionData);

  // Injetar cookies se existirem no session.json
  if (sessionData && sessionData.cookies) {
    try {
      if (Array.isArray(sessionData.cookies)) {
        const normalized = sessionData.cookies.map(c => ({
          name: c.name,
          value: c.value,
          domain: c.domain ? c.domain.replace(/^\./, '') : 'huntera.com.br',
          path: c.path || '/'
        }));
        await page.setCookie(...normalized);
      } else if (typeof sessionData.cookies === 'string') {
        const parsedCookies = sessionData.cookies.split(';').map(c => {
          const parts = c.trim().split('=');
          return {
            name: parts[0],
            value: parts.slice(1).join('='),
            domain: 'huntera.com.br',
            path: '/'
          };
        }).filter(c => c.name && c.value);
        if (parsedCookies.length) {
          await page.setCookie(...parsedCookies);
        }
      }
      ui.addLog('Cookies de autenticação injetados', 'ok');
    } catch (e) {
      ui.addLog(`Aviso cookies: ${e.message}`, 'warn');
    }
  }

  // Injetar motor bot-core.js em cada carregamento/navegação
  await page.evaluateOnNewDocument(botCoreCode);

  ui.addLog(`Navegando para ${config.gameUrl || 'https://huntera.com.br/game'}…`, 'info');

  try {
    await page.goto(config.gameUrl || 'https://huntera.com.br/game', {
      waitUntil: 'domcontentloaded',
      timeout: 60000
    });
  } catch (e) {
    ui.addLog(`Navegação inicial: ${e.message}`, 'warn');
  }

  // Watchdog: Se a página recarregar ou desconectar, garantir que o bot está rodando
  page.on('load', async () => {
    ui.addLog('Página recarregada, verificando bot…', 'info');
    try {
      await page.evaluate(botCoreCode);
    } catch (_) {}
  });

  page.on('error', (err) => {
    ui.addLog(`Erro na página: ${err.message}`, 'err');
  });

  // Encerramento limpo via Ctrl+C (SIGINT) ou SIGTERM
  const cleanup = async () => {
    ui.addLog('Encerrando bot e liberando processos…', 'warn');
    try { await browser.close(); } catch (_) {}
    process.exit(0);
  };

  process.on('SIGINT', cleanup);
  process.on('SIGTERM', cleanup);
}

launchBot().catch((err) => {
  console.error('[Fatal Error]', err);
  process.exit(1);
});
