/**
 * protocol.js
 * Codificador e decodificador binário do protocolo WebSocket do Huntera (/game-socket).
 * Permite rodar o jogo em MODO TERMINAL PURO (Sem navegador, sem Chromium) gastando apenas ~30MB de RAM.
 */

const zlib = require('zlib');
const opcodes = require('../protocol-opcodes.json');

const Ze = 1213550164;
const COMPRESS_FLAG = 1;
const BATCH_FLAG = 2;

// Mapa reverso para decodificar incoming por número de opcode
const INCOMING_BY_CODE = new Map();
for (const [name, code] of Object.entries(opcodes.incoming)) {
  INCOMING_BY_CODE.set(code, name);
}

// Mapa de saída para envio (name -> opcode)
const OUTGOING_BY_NAME = opcodes.outgoing;

/**
 * Cifra XORShift de 32 bits original do motor do Huntera
 */
function xorCipher(r, seed) {
  let t = (seed ^ Ze) >>> 0;
  if (t === 0) t = Ze;
  const view = new DataView(r.buffer, r.byteOffset, r.byteLength);
  const alignedLen = r.length & -4;
  let a = 0;
  for (; a < alignedLen; a += 4) {
    t ^= t << 13; t >>>= 0;
    t ^= t >>> 17;
    t ^= t << 5; t >>>= 0;
    view.setUint32(a, view.getUint32(a, true) ^ t, true);
  }
  if (a < r.length) {
    t ^= t << 13; t >>>= 0;
    t ^= t >>> 17;
    t ^= t << 5; t >>>= 0;
    for (; a < r.length; a++) {
      r[a] ^= (t >>> ((a & 3) << 3)) & 255;
    }
  }
}

/**
 * Codifica uma mensagem para envio no WebSocket
 */
function encodePacket(type, payload = {}) {
  const code = OUTGOING_BY_NAME[type];
  if (code === undefined) {
    throw new Error(`Tipo de mensagem de envio desconhecido: ${type}`);
  }

  const cleanPayload = Object.assign({}, payload);
  delete cleanPayload.type;

  const jsonStr = JSON.stringify([code, cleanPayload]);
  let data = Buffer.from(jsonStr, 'utf8');
  let flags = 0;

  // Compactar com DEFLATE se for grande (>= 8KB)
  if (data.length >= 8192) {
    data = zlib.deflateRawSync(data, { level: 3 });
    flags = COMPRESS_FLAG;
  }

  const seed = (Math.random() * 4294967296) >>> 0;
  const out = Buffer.alloc(5 + data.length);
  out.writeUInt32LE(seed, 0);
  out[4] = flags;
  data.copy(out, 5);

  xorCipher(out.subarray(4), seed);
  return out;
}

/**
 * Decodifica um pacote bruto recebido do WebSocket
 */
function decodeSinglePacket(buf) {
  if (buf.length < 5) return null;
  const seed = buf.readUInt32LE(0);
  const body = Buffer.from(buf.subarray(4));
  xorCipher(body, seed);

  const flags = body[0];
  let payload = body.subarray(1);

  if (flags & COMPRESS_FLAG) {
    try {
      payload = zlib.inflateRawSync(payload);
    } catch (_) {
      return null;
    }
  }

  try {
    const parsed = JSON.parse(payload.toString('utf8'));
    if (!Array.isArray(parsed) || parsed.length !== 2) return null;
    const [code, data] = parsed;
    const type = INCOMING_BY_CODE.get(code) || `unknown_${code}`;
    return Object.assign({ type }, data || {});
  } catch (_) {
    return null;
  }
}

/**
 * Decodifica o quadro recebido (podendo conter 1 ou lote de subpacotes)
 */
function decodeFrame(raw) {
  const buf = Buffer.isBuffer(raw) ? raw : Buffer.from(raw);
  if (buf.length < 5) return [];

  const seed = buf.readUInt32LE(0);
  const body = Buffer.from(buf.subarray(4));
  xorCipher(body, seed);

  const flags = body[0];
  const content = body.subarray(1);

  // Se não for lote (batch)
  if ((flags & BATCH_FLAG) === 0) {
    let payload = content;
    if (flags & COMPRESS_FLAG) {
      try { payload = zlib.inflateRawSync(payload); } catch (_) { return []; }
    }
    try {
      const parsed = JSON.parse(payload.toString('utf8'));
      if (Array.isArray(parsed) && parsed.length === 2) {
        const type = INCOMING_BY_CODE.get(parsed[0]) || `unknown_${parsed[0]}`;
        return [Object.assign({ type }, parsed[1] || {})];
      }
    } catch (_) {}
    return [];
  }

  // Se for lote (batch com múltiplos pacotes com prefixo uint32 de tamanho)
  const results = [];
  let t = 0;
  while (t + 4 <= content.length) {
    const len = content.readUInt32LE(t);
    t += 4;
    if (t + len > content.length) break;
    const sub = content.subarray(t, t + len);
    const item = decodeSinglePacket(sub);
    if (item) results.push(item);
    t += len;
  }
  return results;
}

module.exports = {
  encodePacket,
  decodeFrame,
  xorCipher,
  CLIENT_VERSION: '0.3.0+e0'
};
