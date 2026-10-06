/**
 * export-session-helper.js
 * 
 * COMO EXPORTAR SUA CONTA DO NAVEGADOR PARA A VPS EM 5 SEGUNDOS:
 * 
 * 1. No seu computador, abra o jogo no navegador (Chrome, Edge, Firefox, Brave, etc):
 *    https://huntera.com.br/game
 * 2. Certifique-se de que está logado no seu personagem.
 * 3. Pressione a tecla F12 para abrir o Console de Desenvolvedor.
 * 4. Cole o código abaixo e aperte ENTER:
 */

(function () {
  const session = {
    cookies: document.cookie,
    localStorage: Object.keys(localStorage).reduce((acc, key) => {
      acc[key] = localStorage.getItem(key);
      return acc;
    }, {})
  };

  const json = JSON.stringify(session, null, 2);

  if (typeof copy === 'function') {
    copy(json);
    console.log('%c[✓] SUCESSO! A sessão foi copiada para sua Área de Transferência (Ctrl+V)!', 'background: #059669; color: white; padding: 6px 12px; font-size: 14px; font-weight: bold; border-radius: 6px;');
  } else {
    console.log(json);
    console.log('%c[i] Copie o JSON acima e cole no arquivo session.json na VPS.', 'background: #2563eb; color: white; padding: 6px 12px; font-size: 14px; font-weight: bold; border-radius: 6px;');
  }
})();
