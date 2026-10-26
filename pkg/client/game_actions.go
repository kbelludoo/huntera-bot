package client

// Convenience game action methods for SocketClient

func (s *SocketClient) ClaimDailyReward() error {
	return s.Send("claim-daily-reward", map[string]any{})
}

func (s *SocketClient) DailyBossOpen() error {
	return s.Send("daily-boss-open", map[string]any{})
}

func (s *SocketClient) DailyBossCreate(tier int) error {
	return s.Send("daily-boss-create", map[string]any{"tier": tier})
}

func (s *SocketClient) DailyBossInvite(charName string) error {
	return s.Send("daily-boss-invite", map[string]any{"name": charName, "characterName": charName})
}

func (s *SocketClient) DailyBossJoin(roomID any) error {
	return s.Send("daily-boss-join", map[string]any{"roomId": roomID})
}

func (s *SocketClient) DailyBossReady(ready bool) error {
	return s.Send("daily-boss-ready", map[string]any{"ready": ready})
}

func (s *SocketClient) DailyBossStart() error {
	return s.Send("daily-boss-start", map[string]any{})
}

func (s *SocketClient) DailyBossClaim() error {
	return s.Send("daily-boss-claim", map[string]any{})
}

func (s *SocketClient) QuestPick(questID any) error {
	return s.Send("quest-pick", map[string]any{"questId": questID})
}

func (s *SocketClient) QuestClaim(questID any) error {
	return s.Send("quest-claim", map[string]any{"questId": questID})
}

func (s *SocketClient) SetActionBarManaged(managed bool) error {
	return s.Send("set-action-bar-managed", map[string]any{"managed": managed})
}

func (s *SocketClient) SelectActionBarPreset(preset string) error {
	return s.Send("select-action-bar-preset", map[string]any{"preset": preset})
}

func (s *SocketClient) SetActionSlot(slot int, action any) error {
	return s.Send("set-action-slot", map[string]any{"slot": slot, "action": action})
}

func (s *SocketClient) PreyPick(slot int, monsterID string) error {
	return s.Send("prey-pick", map[string]any{"slot": slot, "monsterId": monsterID})
}

func (s *SocketClient) PreyListReroll(slot int) error {
	return s.Send("prey-list-reroll", map[string]any{"slot": slot})
}

func (s *SocketClient) ForgeAutoConvert(enabled bool) error {
	return s.Send("forge-auto-convert", map[string]any{"enabled": enabled})
}

func (s *SocketClient) ForgeConvert(amount int) error {
	return s.Send("forge-convert", map[string]any{"amount": amount})
}

func (s *SocketClient) ShopOpen() error {
	return s.Send("shop-open", map[string]any{})
}

func (s *SocketClient) ShopBuy(itemID int, amount int) error {
	return s.Send("shop-buy", map[string]any{"itemId": itemID, "amount": amount})
}

func (s *SocketClient) EquipItem(container string, index int) error {
	return s.Send("equip-item", map[string]any{
		"ref": map[string]any{
			"container": container,
			"index":     index,
		},
	})
}

func (s *SocketClient) SetAmmoRules(rules any) error {
	return s.Send("set-ammo-rules", map[string]any{"rules": rules})
}

func (s *SocketClient) SelectAmmo(itemID int) error {
	return s.Send("select-ammo", map[string]any{"itemId": itemID})
}

func (s *SocketClient) StoreOpen() error {
	return s.Send("store-open", map[string]any{})
}

func (s *SocketClient) StoreBuy(offerID any) error {
	return s.Send("store-buy", map[string]any{
		"offerId": offerID,
		"id":      offerID,
	})
}

func (s *SocketClient) StoreHistoryRequest() error {
	return s.Send("store-history-request", map[string]any{})
}

func (s *SocketClient) CoinsRefresh() error {
	return s.Send("coins-refresh", map[string]any{})
}

func (s *SocketClient) MarketBrowse(category string) error {
	return s.Send("market-browse", map[string]any{"category": category})
}

func (s *SocketClient) MarketAcceptOffer(offerID any, amount int) error {
	return s.Send("market-accept-offer", map[string]any{
		"offerId": offerID,
		"amount":  amount,
	})
}

func (s *SocketClient) CoinTransfer(recipient string, amount int) error {
	return s.Send("coin-transfer", map[string]any{
		"recipient":     recipient,
		"characterName": recipient,
		"name":          recipient,
		"target":        recipient,
		"amount":        amount,
	})
}

func (s *SocketClient) TradeRequest(targetName string) error {
	return s.Send("trade-request", map[string]any{
		"characterName": targetName,
		"name":          targetName,
		"target":        targetName,
	})
}

func (s *SocketClient) TradeRespond(accept bool) error {
	return s.Send("trade-respond", map[string]any{
		"accept": accept,
	})
}

func (s *SocketClient) TradeSetGold(amount int) error {
	return s.Send("trade-set-gold", map[string]any{
		"amount": amount,
		"gold":   amount,
	})
}

func (s *SocketClient) TradeReady(ready bool) error {
	return s.Send("trade-ready", map[string]any{
		"ready": ready,
	})
}

func (s *SocketClient) TradeConfirm(confirm bool) error {
	return s.Send("trade-confirm", map[string]any{
		"confirm": confirm,
	})
}

func (s *SocketClient) TradeCancel() error {
	return s.Send("trade-cancel", map[string]any{})
}


