package bot

import (
	"log"
	"strings"
	"time"

	"huntera-bot/pkg/client"
	"huntera-bot/pkg/protocol"
)

// setupGoldPool initializes event listeners for automated party gold and coin pooling.
func (b *Bot) setupGoldPool(sock *client.SocketClient) {
	// 🤝 Pedido de Trade recebido de outro personagem
	sock.On("trade-requested", func(ev protocol.RawEvent) {
		b.handleTradeRequested(sock, ev)
	})

	// 🤝 Sessão de Trade iniciada (ambos conectados na janela de troca)
	sock.On("trade-session", func(ev protocol.RawEvent) {
		b.handleTradeSession(sock, ev)
	})

	// 🤝 Fechamento / Conclusão de Trade
	sock.On("trade-closed", func(_ protocol.RawEvent) {
		b.mu.Lock()
		b.tradeInProgress = false
		b.mu.Unlock()
	})

	// 🪙 Resultado de transferência direta de moedas (Coins)
	sock.On("coin-transfer-result", func(ev protocol.RawEvent) {
		log.Printf("[%s: %s] 🪙 [COIN POOL] Resposta de transferência: %s", b.config.ID, b.status.CharName, string(ev.Payload))
		_ = sock.CoinsRefresh()
	})
}

func (b *Bot) handleTradeRequested(sock *client.SocketClient, ev protocol.RawEvent) {
	var data struct {
		Name          string `json:"name"`
		CharacterName string `json:"characterName"`
	}
	_ = ev.Unmarshal(&data)
	requester := data.CharacterName
	if requester == "" {
		requester = data.Name
	}

	// Regra de Segurança: Aceita trade APENAS se o solicitante for um membro registrado da nossa party
	if !GlobalCoordinator.IsTeamMember(requester) {
		log.Printf("[%s: %s] ⚠️ [TRADE] Recusando trade de jogador desconhecido: %s", b.config.ID, b.status.CharName, requester)
		_ = sock.TradeRespond(false)
		return
	}

	log.Printf("[%s: %s] 🤝 [TRADE] Aceitando pedido de Trade do parceiro de party: %s ✓", b.config.ID, b.status.CharName, requester)
	b.mu.Lock()
	b.tradeInProgress = true
	b.tradePartner = requester
	b.mu.Unlock()

	_ = sock.TradeRespond(true)
}

func (b *Bot) handleTradeSession(sock *client.SocketClient, ev protocol.RawEvent) {
	var data struct {
		PartnerName string `json:"partnerName"`
		Name        string `json:"name"`
	}
	_ = ev.Unmarshal(&data)
	partner := data.PartnerName
	if partner == "" {
		partner = data.Name
	}
	if partner == "" {
		b.mu.RLock()
		partner = b.tradePartner
		b.mu.RUnlock()
	}

	b.mu.RLock()
	gold := b.status.Gold
	myChar := b.status.CharName
	b.mu.RUnlock()

	vipChar, vipAccID := GlobalCoordinator.GetVipTarget()
	isDonor := b.config.ID != vipAccID && !strings.EqualFold(myChar, vipChar)

	go func() {
		time.Sleep(300 * time.Millisecond)

		if isDonor {
			// Conta Doadora: reserva 20.000 gp para suprimentos e envia todo o excedente
			surplus := gold - 20000
			if surplus > 0 {
				log.Printf("[%s: %s] 💰 [GOLD POOL] Colocando %d gp na janela de troca para %s…",
					b.config.ID, myChar, surplus, partner)
				_ = sock.TradeSetGold(surplus)
				time.Sleep(400 * time.Millisecond)
			}
		}

		// Ambas as contas sinalizam Ready e depois Confirm
		_ = sock.TradeReady(true)
		time.Sleep(400 * time.Millisecond)
		_ = sock.TradeConfirm(true)
		log.Printf("[%s: %s] 🤝 [TRADE] Troca confirmada com sucesso com %s! ✓", b.config.ID, myChar, partner)
	}()
}

// checkAndInitiateGoldPool verifica periodicamente se a conta deve repassar ouro ou moedas para o alvo VIP atual
func (b *Bot) checkAndInitiateGoldPool(sock *client.SocketClient) {
	b.mu.RLock()
	inGame := b.status.InGame
	inHunt := b.status.Hunt != nil
	gold := b.status.Gold
	coins := b.status.Coins
	myChar := b.status.CharName
	lastTrade := b.lastTradeAttempt
	tradeActive := b.tradeInProgress
	b.mu.RUnlock()

	if !inGame || inHunt || tradeActive {
		return
	}

	vipChar, vipAccID := GlobalCoordinator.GetVipTarget()
	if vipChar == "" || b.config.ID == vipAccID || strings.EqualFold(myChar, vipChar) {
		// Esta conta é o alvo atual de recebimento de VIP; não doa
		return
	}

	// 1. Repasse Direto de Coins: Se este follower tem moedas acumuladas, transfere para o alvo VIP
	if coins > 0 {
		now := time.Now()
		if now.Sub(lastTrade) > 30*time.Second {
			b.mu.Lock()
			b.lastTradeAttempt = now
			b.mu.Unlock()

			log.Printf("[%s: %s] 🪙 [COIN POOL] Transferindo %d Coins acumuladas diretamente para o alvo VIP (%s)…",
				b.config.ID, myChar, coins, vipChar)
			_ = sock.CoinTransfer(vipChar, coins)
			time.Sleep(500 * time.Millisecond)
			_ = sock.CoinsRefresh()
			return
		}
	}

	// 2. Repasse de Ouro: Se a conta tem excedente significativo (>= 15k acima da reserva de 20k)
	surplus := gold - 20000
	if surplus >= 15000 {
		now := time.Now()
		if now.Sub(lastTrade) > 60*time.Second {
			b.mu.Lock()
			b.lastTradeAttempt = now
			b.tradeInProgress = true
			b.tradePartner = vipChar
			b.mu.Unlock()

			log.Printf("[%s: %s] 🤝 [GOLD POOL] Solicitando Trade com %s para transferir %d gp de excedente para a compra da VIP…",
				b.config.ID, myChar, vipChar, surplus)
			_ = sock.TradeRequest(vipChar)
		}
	}
}
