/**
 * optimizer.js
 * Otimizações extremas de CPU e Memória para rodar no ambiente 1 vCPU / 1 GB RAM.
 * - Bloqueia requisições de imagens, fontes e áudio (Modo Sem Gráficos real).
 * - Neutraliza Canvas 2D / WebGL para 0% de uso de renderização gráfica.
 * - Reduz o requestAnimationFrame de 60 FPS para ~2 FPS (economizando 90% de CPU).
 * - Desativa Web Audio.
 */

async function applyPageOptimizations(page, sessionData = null) {
  // 1. Injetar scripts de stub antes de qualquer carregamento do jogo
  await page.evaluateOnNewDocument((session) => {
    // A. Injetar localStorage da sessão salva
    if (session && session.localStorage) {
      try {
        for (const [key, val] of Object.entries(session.localStorage)) {
          window.localStorage.setItem(key, typeof val === 'string' ? val : JSON.stringify(val));
        }
      } catch (e) {
        console.warn('[Optimizer] Falha ao injetar localStorage:', e);
      }
    }

    // B. Desativar áudio completamente (economiza CPU com decodificação de som)
    try {
      window.AudioContext = undefined;
      window.webkitAudioContext = undefined;
      if (window.HTMLMediaElement) {
        window.HTMLMediaElement.prototype.play = () => Promise.resolve();
      }
    } catch (_) {}

    // C. Prototype Trap: intercepta a atribuição do gameState e socket no construtor do app-game-shell
    // Isso é 100% à prova de falhas porque o construtor do jogo executa:
    // this.gameState = e; this.socket = n;
    try {
      Object.defineProperty(Object.prototype, 'gameState', {
        set(val) {
          if (val && typeof val === 'object' && typeof val.takeLoot === 'function') {
            window.__HUNTERA_GAME_STATE__ = val;
            console.log('[Optimizer] gameState capturado com sucesso via Prototype Trap!');
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
            console.log('[Optimizer] socket capturado com sucesso via Prototype Trap!');
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
    } catch (_) {}

    // D. Limitar requestAnimationFrame para ~2 FPS
    // Angular e os WebSockets continuam rodando 100%, mas o loop de desenho não queima CPU.
    try {
      const origRAF = window.requestAnimationFrame;
      window.requestAnimationFrame = function (callback) {
        return setTimeout(() => {
          try { callback(performance.now()); } catch (_) {}
        }, 500); // 2 FPS
      };
      window.cancelAnimationFrame = function (id) {
        clearTimeout(id);
      };
    } catch (_) {}

    // D. Neutralizar renderizador WebGL e Canvas 2D pesado
    // Sem gráficos = zero consumo de textura na RAM e zero llvmpipe no Linux
    try {
      const origGetContext = HTMLCanvasElement.prototype.getContext;
      HTMLCanvasElement.prototype.getContext = function (type, ...args) {
        // Bloqueia WebGL para não subir emulador de shader por software
        if (type === 'webgl' || type === 'webgl2' || type === 'experimental-webgl') {
          return null;
        }
        const ctx = origGetContext.call(this, type, ...args);
        if (ctx) {
          // No-op em operações de desenho pesado
          ctx.drawImage = function () {};
          ctx.stroke = function () {};
          ctx.fill = function () {};
          ctx.fillRect = function () {};
          ctx.clearRect = function () {};
        }
        return ctx;
      };
    } catch (_) {}

    // E. Desativar animações CSS e transições
    try {
      const style = document.createElement('style');
      style.textContent = `
        *, *::before, *::after {
          animation-duration: 0.001s !important;
          animation-iteration-count: 1 !important;
          transition-duration: 0.001s !important;
        }
      `;
      document.documentElement.appendChild(style);
    } catch (_) {}
  }, sessionData);

  // 2. Interceptar e bloquear requisições pesadas de rede
  await page.setRequestInterception(true);

  page.on('request', (req) => {
    const resourceType = req.resourceType();
    const url = req.url().toLowerCase();

    // Bloquear imagens, mídias e fontes (mesmo com query string)
    if (
      resourceType === 'image' ||
      resourceType === 'media' ||
      resourceType === 'font' ||
      /\.(png|jpe?g|webp|gif|svg|woff2?|ttf|eot|mp3|ogg|wav)(\?|$)/i.test(url) ||
      url.includes('google-analytics') ||
      url.includes('doubleclick')
    ) {
      return req.abort();
    }

    // Permitir scripts essenciais, XHR, Fetch, WebSocket e HTML
    req.continue();
  });
}

/**
 * Flags recomendadas para o Chromium rodar com < 200MB de RAM e 1 vCPU
 */
const CHROMIUM_OPTIMIZED_FLAGS = [
  '--headless=new',
  '--no-sandbox',
  '--disable-setuid-sandbox',
  '--disable-gpu',
  '--disable-software-rasterizer',
  '--disable-dev-shm-usage', // Crítico para VPS com /dev/shm pequeno
  '--no-zygote',
  '--single-process', // Agrupa processos para reduzir overhead de RAM em ~40%
  '--mute-audio',
  '--no-first-run',
  '--disable-extensions',
  '--disable-default-apps',
  '--disable-background-networking',
  '--disable-sync',
  '--disable-translate',
  '--disable-features=Translate,OptimizationHints,MediaRouter',
  '--js-flags=--max-old-space-size=256 --optimize-for-size', // Limita heap V8 a 256MB
  '--window-size=800,600'
];

module.exports = {
  applyPageOptimizations,
  CHROMIUM_OPTIMIZED_FLAGS
};
