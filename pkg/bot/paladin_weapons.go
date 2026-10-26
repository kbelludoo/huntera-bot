package bot

import (
	"log"
	"time"

	"huntera-bot/pkg/client"
)

// setupPaladinAmmoRules ensures the paladin shoots from 100% to 0% HP without health locks
func (b *Bot) setupPaladinAmmoRules(sock *client.SocketClient) {
	_ = sock.SetAmmoRules([]any{
		map[string]any{
			"itemId": 21470, // Simple Arrow
			"conditions": []any{
				map[string]any{
					"subject":   "target",
					"attribute": "health",
					"operator":  ">=",
					"value":     0,
					"percent":   true,
				},
			},
		},
		map[string]any{
			"itemId": 3447, // Arrow regular
			"conditions": []any{
				map[string]any{
					"subject":   "target",
					"attribute": "health",
					"operator":  ">=",
					"value":     0,
					"percent":   true,
				},
			},
		},
	})
	_ = sock.SelectAmmo(21470)
}

// handlePaladinCitySupplies ensures the paladin buys and carries distance weapons (spears/bows)
func (b *Bot) handlePaladinCitySupplies(sock *client.SocketClient) {
	b.setupPaladinAmmoRules(sock)

	// Se já verificou suprimentos recentemente (nos últimos 10 minutos), evita chamadas redundantes
	now := time.Now()
	b.mu.Lock()
	if now.Sub(b.lastPaladinEquipAttempt) < 10*time.Minute {
		b.mu.Unlock()
		return
	}
	b.lastPaladinEquipAttempt = now
	b.mu.Unlock()

	log.Printf("[%s: %s] 🏹 [PALADINO] Na cidade: verificando suprimentos de armas de distância (spears)…",
		b.config.ID, b.status.CharName)
	_ = sock.ShopOpen()
	time.Sleep(200 * time.Millisecond)
	_ = sock.ShopBuy(3277, 5) // 5x Spear
}

// handlePaladinEquip prioritizes distance weapons (spear, bow) for the paladin over melee swords/wands
func (b *Bot) handlePaladinEquip(sock *client.SocketClient, curWeap *InventorySlot, slots []*InventorySlot) {
	isDistanceEquipped := curWeap != nil && (curWeap.AttackSkill == "distance" || curWeap.ItemID == 3277 || curWeap.ItemID == 21427)

	// Se já está com arma de distância equipada, nada a fazer
	if isDistanceEquipped {
		return
	}

	bestIdx := -1
	bestAtk := -1
	bestName := ""

	// Prioriza Spear (ItemId 3277) ou armas com attackSkill "distance"
	for idx, s := range slots {
		if s == nil || s.Slot != "weapon" {
			continue
		}
		// Lança tem prioridade absoluta pois não consome flechas do quiver
		if s.ItemID == 3277 {
			bestIdx = idx
			bestAtk = 9
			bestName = s.Name
			break
		}
		if s.AttackSkill == "distance" && s.Attack >= bestAtk {
			bestIdx = idx
			bestAtk = s.Attack
			bestName = s.Name
		}
	}

	if bestIdx >= 0 {
		log.Printf("[%s: %s] 🏹 [PALADINO AUTO-EQUIP] Equipando arma de distância %s (Atk: %d, Backpack Index: %d)…",
			b.config.ID, b.status.CharName, bestName, bestAtk, bestIdx)
		_ = sock.EquipItem("backpack", bestIdx)
	}
}
