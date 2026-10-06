const { httpRequest } = require('./src/term/session');

async function test() {
  const email = 'kbelludoo@gmail.com';
  const password = 'ECTbb,,0209';

  console.log('Testando POST /api/auth/login...');
  try {
    const res = await httpRequest('/api/auth/login', {
      method: 'POST',
      body: { email, password }
    });
    console.log('Status:', res.status);
    console.log('Data:', res.data);
    console.log('Set-Cookie:', res.headers['set-cookie']);
  } catch (e) {
    console.error('Erro:', e.message, e.data);
  }
}

test();
