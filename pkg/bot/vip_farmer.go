package bot

import (
	"log"
	"strings"
	"time"

	"huntera-bot/pkg/client"
	"huntera-bot/pkg/protocol"
)

// ProfitHuntTier defines the gold/h profit hierarchy for VIP farming
type ProfitHuntTier struct {
	MinLevel            int
	HuntID              string
	Name                string
	ExpectedGoldPerHour int
}

var ProfitHuntHierarchy = []ProfitHuntTier{
	{MinLevel: 150, HuntID: "asura-hunt", Name: "Asura Palace (Endgame Profit)", ExpectedGoldPerHour: 500000},
	{MinLevel: 120, HuntID: "hero-hunt", Name: "Hero Cave (Crown Gear & High EXP)", ExpectedGoldPerHour: 350000},
	{MinLevel: 80, HuntID: "mutated-cave-hunt", Name: "Mutated Cave (Sabretooth 15k+)", ExpectedGoldPerHour: 250000},
	{MinLevel: 65, HuntID: "cult-hunt", Name: "Cultists (Rope Belts 8k-15k)", ExpectedGoldPerHour: 180000},
	{MinLevel: 25, HuntID: "tarantula-hunt", Name: "Tarantula Caves (Eggs & Fangs)", ExpectedGoldPerHour: 80000},
	{MinLevel: 20, HuntID: "amazon-hunt", Name: "Amazon Camp (Protective Charms)", ExpectedGoldPerHour: 60000},
	{MinLevel: 10, HuntID: "swamp-troll-hunt", Name: "Swamp Trolls (Medicine Pouches)", ExpectedGoldPerHour: 40000},
}

// GetBestProfitHunt returns the top profit cave for the team's level
func GetBestProfitHunt(teamMinLevel int) (huntID, name string) {
	for _, tier := range ProfitHuntHierarchy {
		if teamMinLevel >= tier.MinLevel {
			return tier.HuntID, tier.Name
		}
	}
	return "cult-hunt", "Cultists (Rope Belts)"
}

// IsVipFarmModeActive returns true if the bot should focus on VIP farming rather than low-tier bestiary rotation
func (b *Bot) IsVipFarmModeActive() bool {
	if b.config.FarmVIPMode != nil && !*b.config.FarmVIPMode {
		return false
	}
	b.mu.RLock()
	vipDays := b.status.VipDays
	b.mu.RUnlock()

	// Permanece em VIP Farm Mode enquanto a equipe ainda não tiver travado os 90 dias de VIP (Fase 2)
	return vipDays < 90
}

func (b *Bot) setupVipFarmer(sock *client.SocketClient) {
	// 🪙 Atualização de saldo de Tibia Coins
	sock.On("coins", func(ev protocol.RawEvent) {
		b.handleCoins(sock, ev)
	})

	// 👑 Status da VIP
	sock.On("vip-status", func(ev protocol.RawEvent) {
		b.handleVipStatus(sock, ev)
	})

	// 🏪 Catálogo de Ofertas da Loja
	sock.On("store-offers", func(ev protocol.RawEvent) {
		b.handleStoreOffers(sock, ev)
	})

	// 🏪 Confirmação de Compra na Loja
	sock.On("store-result", func(ev protocol.RawEvent) {
		b.handleStoreResult(sock, ev)
	})

	// 📊 Leilão / Mercado
	sock.On("market-browse-result", func(ev protocol.RawEvent) {
		b.handleMarketBrowseResult(sock, ev)
	})
}

func (b *Bot) handleCoins(sock *client.SocketClient, ev protocol.RawEvent) {
	var data struct {
		Coins           int `json:"coins"`
		Amount          int `json:"amount"`
		Balance         int `json:"balance"`
		Transferable    int `json:"transferable"`
		NonTransferable int `json:"nonTransferable"`
	}
	if err := ev.Unmarshal(&data); err != nil {
		return
	}

	total := data.Coins
	if total == 0 {
		total = data.Amount
	}
	if total == 0 {
		total = data.Balance
	}
	if total == 0 {
		total = data.Transferable + data.NonTransferable
	}

	b.mu.Lock()
	oldCoins := b.status.Coins
	b.status.Coins = total
	vipDays := b.status.VipDays
	b.mu.Unlock()

	if total != oldCoins {
		log.Printf("[%s: %s] 🪙 [COINS] Saldo atualizado: %d coins (Anterior: %d)", b.config.ID, b.status.CharName, total, oldCoins)
	}

	// Gatilho autônomo de compra de VIP para o líder da equipe
	if b.config.Role == "leader" {
		if vipDays <= 0 && total >= 250 {
			// FASE 1: Destravar a primeira VIP de 30 dias
			b.buyVipTier(sock, 30, 250, "premium-30")
		} else if vipDays > 0 && vipDays <= 30 && total >= 650 {
			// FASE 2: Travar a segunda VIP de 90 dias com desconto de 100 coins
			b.buyVipTier(sock, 90, 650, "premium-90")
		}
	}
}

func (b *Bot) handleVipStatus(sock *client.SocketClient, ev protocol.RawEvent) {
	var data struct {
		Active   bool  `json:"active"`
		Days     int   `json:"days"`
		VipDays  int   `json:"vipDays"`
		ExpireAt int64 `json:"expireAt"`
	}
	if err := ev.Unmarshal(&data); err != nil {
		return
	}

	days := data.Days
	if days == 0 {
		days = data.VipDays
	}
	if days == 0 && data.ExpireAt > 0 {
		nowSec := time.Now().Unix()
		if data.ExpireAt > nowSec {
			days = int((data.ExpireAt - nowSec) / 86400)
		}
	}
	active := data.Active || days > 0

	b.mu.Lock()
	prevDays := b.status.VipDays
	b.status.VipDays = days
	b.status.VipActive = active
	b.mu.Unlock()

	if prevDays != days {
		log.Printf("[%s: %s] 👑 [VIP STATUS] Status VIP atualizado: %d dias restantes (Ativo: %v)",
			b.config.ID, b.status.CharName, days, active)
	}
}

func (b *Bot) handleStoreOffers(sock *client.SocketClient, ev protocol.RawEvent) {
	var data struct {
		Offers []struct {
			ID          any    `json:"id"`
			OfferID     any    `json:"offerId"`
			Name        string `json:"name"`
			Title       string `json:"title"`
			Price       int    `json:"price"`
			Coins       int    `json:"coins"`
			Description string `json:"description"`
		} `json:"offers"`
	}
	if err := ev.Unmarshal(&data); err != nil {
		return
	}

	b.mu.RLock()
	coins := b.status.Coins
	vipDays := b.status.VipDays
	role := b.config.Role
	b.mu.RUnlock()

	if role != "leader" {
		return
	}

	for _, offer := range data.Offers {
		price := offer.Price
		if price == 0 {
			price = offer.Coins
		}
		nameLower := strings.ToLower(offer.Name + " " + offer.Title)

		targetID := offer.OfferID
		if targetID == nil {
			targetID = offer.ID
		}

		// Checa se é a VIP de 30 dias (Fase 1: 250 coins)
		if vipDays <= 0 && coins >= 250 && (price == 250 || strings.Contains(nameLower, "30")) {
			log.Printf("[%s: %s] 🎯 [STORE] Oferta VIP de 30 dias localizada (#%v: %s por %d coins)! Executando compra…",
				b.config.ID, b.status.CharName, targetID, offer.Name, price)
			_ = sock.StoreBuy(targetID)
			time.Sleep(300 * time.Millisecond)
			_ = sock.CoinsRefresh()
			return
		}

		// Checa se é a VIP de 90 dias (Fase 2: 650 coins)
		if vipDays > 0 && vipDays <= 30 && coins >= 650 && (price == 650 || strings.Contains(nameLower, "90")) {
			log.Printf("[%s: %s] 🎯 [STORE] Oferta VIP de 90 dias localizada (#%v: %s por %d coins)! Executando compra…",
				b.config.ID, b.status.CharName, targetID, offer.Name, price)
			_ = sock.StoreBuy(targetID)
			time.Sleep(300 * time.Millisecond)
			_ = sock.CoinsRefresh()
			return
		}
	}
}

func (b *Bot) handleStoreResult(sock *client.SocketClient, ev protocol.RawEvent) {
	log.Printf("[%s: %s] 🏪 [STORE RESULT] Resposta da Loja: %s", b.config.ID, b.status.CharName, string(ev.Payload))
	_ = sock.CoinsRefresh()
}

func (b *Bot) buyVipTier(sock *client.SocketClient, days int, cost int, fallbackOffer string) {
	log.Printf("================================================================================")
	log.Printf("🎉 [%s: %s] 🚀 [VIP AUTÔNOMO - FASE %d] %d COINS ATINGIDOS!", b.config.ID, b.status.CharName, map[bool]int{true: 1, false: 2}[days == 30], cost)
	log.Printf("👑 ATIVANDO VIP DE %d DIAS NA CONTA PRINCIPAL! (+10%% EXP PERMANENTE E PARTY ILIMITADA)", days)
	log.Printf("================================================================================")

	_ = sock.StoreOpen()
	time.Sleep(200 * time.Millisecond)
	_ = sock.StoreBuy(fallbackOffer)
	time.Sleep(300 * time.Millisecond)
	_ = sock.CoinsRefresh()
}

func (b *Bot) handleMarketBrowseResult(sock *client.SocketClient, ev protocol.RawEvent) {
	var data struct {
		Offers []struct {
			OfferID int    `json:"offerId"`
			ID      any    `json:"id"`
			ItemID  int    `json:"itemId"`
			Name    string `json:"name"`
			Amount  int    `json:"amount"`
			Price   int    `json:"price"`
			Kind    string `json:"kind"`
		} `json:"offers"`
	}
	if err := ev.Unmarshal(&data); err != nil {
		return
	}

	b.mu.RLock()
	gold := b.status.Gold
	role := b.config.Role
	b.mu.RUnlock()

	if role != "leader" || gold < 100000 {
		return
	}

	for _, offer := range data.Offers {
		nameLower := strings.ToLower(offer.Name)
		if strings.Contains(nameLower, "coin") && (offer.Kind == "sell" || offer.Kind == "") {
			totalCost := offer.Price * offer.Amount
			// Se o custo total for comportado pelo ouro com reserva de 50k gp
			if totalCost > 0 && totalCost <= (gold-50000) {
				log.Printf("[%s: %s] 🪙 [MARKET] Comprando %dx Coins por %d gp total (%d gp cada)…",
					b.config.ID, b.status.CharName, offer.Amount, totalCost, offer.Price)
				targetID := offer.OfferID
				if targetID == 0 && offer.ID != nil {
					targetID, _ = offer.ID.(int)
				}
				_ = sock.MarketAcceptOffer(targetID, offer.Amount)
				time.Sleep(200 * time.Millisecond)
				_ = sock.CoinsRefresh()
				break
			}
		}
	}
}
