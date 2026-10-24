package bot

import (
	"log"
	"time"

	"huntera-bot/pkg/client"
)

// handleMarketBatchSell automatically lists surplus protected high-value items on the auction market
func (b *Bot) handleMarketBatchSell(sock *client.SocketClient, slots []*InventorySlot) {
	b.mu.RLock()
	aucPrices := b.auctionPrices
	b.mu.RUnlock()

	if len(aucPrices) == 0 {
		return
	}

	for _, slot := range slots {
		if slot == nil || slot.Count < 5 {
			continue
		}
		price := aucPrices[slot.ItemID]
		// Se for item valioso com preço registrado no leilão (>= 5000 gp)
		if price >= 5000 {
			sellCount := slot.Count / 2 // vende metade para manter estoque de imbuement
			if sellCount > 0 {
				log.Printf("[%s: %s] 💰 [AUTO-MARKET] Criando oferta de %dx %s por %d gp cada no Mercado…",
					b.config.ID, b.status.CharName, sellCount, slot.Name, price)
				_ = sock.MarketOpen()
				time.Sleep(200 * time.Millisecond)
				_ = sock.MarketCreateOffer(slot.ItemID, sellCount, price)
				time.Sleep(100 * time.Millisecond)
				_ = sock.MarketClose()
			}
		}
	}
}
