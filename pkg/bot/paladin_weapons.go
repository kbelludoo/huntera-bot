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

	log.Printf("[%s: %s] 🏹 [PALADINO] Na cidade: garantindo suprimentos de armas de distância (spears / paladin's bow)…",
		b.config.ID, b.status.CharName)
	_ = sock.ShopOpen()
	time.Sleep(200 * time.Millisecond)
	_ = sock.ShopBuy(3277, 10) // 10x Spear (10 gp cada - lança com Atk 9 nativo, sem necessidade de flechas)
	_ = sock.ShopBuy(21427, 1) // 1x Paladin's Bow (0 gp)
	_ = sock.Send("tutorial-weapon", map[string]any{"weapon": "spear"})
	_ = sock.Send("tutorial-weapon", map[string]any{"weapon": "bow"})
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
