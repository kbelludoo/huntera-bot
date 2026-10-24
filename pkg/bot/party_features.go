package bot

import (
	"log"
	"time"

	"huntera-bot/pkg/client"
	"huntera-bot/pkg/protocol"
)

// claimDailyReward requests daily reward upon login
func (b *Bot) claimDailyReward(sock *client.SocketClient) {
	log.Printf("[%s: %s] 🎁 [DAILY REWARD] Solicitando recompensa diária…", b.config.ID, b.status.CharName)
	_ = sock.ClaimDailyReward()
}

// setupPartyFeatures initializes all automated party, quest, boss, and combat systems
func (b *Bot) setupPartyFeatures(sock *client.SocketClient) {
	// 🎁 Daily Reward
	sock.On("daily-reward-offer", func(_ protocol.RawEvent) {
		log.Printf("[%s: %s] 🎁 [DAILY REWARD] Recompensa diária disponível! Coletando…", b.config.ID, b.status.CharName)
		_ = sock.ClaimDailyReward()
	})

	// 📜 Quests Diárias
	sock.On("quest-status", func(ev protocol.RawEvent) {
		b.handleQuestStatus(sock, ev)
	})
	sock.On("quest-victory", func(ev protocol.RawEvent) {
		log.Printf("[%s: %s] 📜 [QUEST] Vitória em missão registrada: %s", b.config.ID, b.status.CharName, string(ev.Payload))
	})

	// 🎯 Sistema Inteligente de Prey
	sock.On("prey-state", func(ev protocol.RawEvent) {
		b.handlePreyState(sock, ev)
	})

	// ⚒️ Forja / Forge Dust Auto-Convert
	sock.On("forge-dust-gained", func(ev protocol.RawEvent) {
		b.handleForgeDust(sock, ev)
	})
	sock.On("forge-state", func(_ protocol.RawEvent) {
		_ = sock.ForgeAutoConvert(true)
	})

	// 🐉 Daily Bosses Coordenados em Grupo (Party)
	sock.On("daily-boss-status", func(ev protocol.RawEvent) {
		b.handleDailyBossStatus(sock, ev)
	})
	sock.On("daily-boss-invited", func(ev protocol.RawEvent) {
		b.handleDailyBossInvited(sock, ev)
	})
	sock.On("daily-boss-rooms", func(ev protocol.RawEvent) {
		b.handleDailyBossRooms(sock, ev)
	})
	sock.On("daily-boss-rewards", func(_ protocol.RawEvent) {
		log.Printf("[%s: %s] 🐉 [DAILY BOSS] Recompensa do Boss disponível! Resgatando…", b.config.ID, b.status.CharName)
		_ = sock.DailyBossClaim()
	})
	sock.On("daily-boss-victory", func(_ protocol.RawEvent) {
		log.Printf("[%s: %s] 🏆 [DAILY BOSS] VITÓRIA CONTRA O BOSS! Resgatando recompensa…", b.config.ID, b.status.CharName)
		_ = sock.DailyBossClaim()
	})
}

// initActionBar configures the managed action bar and vocation rotation
func (b *Bot) initActionBar(sock *client.SocketClient) {
	_ = sock.SetActionBarManaged(true)

	vocation := "paladin"
	switch b.config.ID {
	case "acc3":
		vocation = "knight"
	case "acc2":
		vocation = "druid"
	case "acc4":
		vocation = "sorcerer"
	}
	_ = sock.SelectActionBarPreset(vocation)
	log.Printf("[%s: %s] ⚔️ [ACTION BAR] Action Bar gerenciada e ativada para vocação: %s ✓", b.config.ID, b.status.CharName, vocation)
}

// handleQuestStatus claims completed quests and picks available daily tasks
func (b *Bot) handleQuestStatus(sock *client.SocketClient, ev protocol.RawEvent) {
	var data struct {
		Quests []struct {
			ID        any    `json:"id"`
			Status    string `json:"status"`
			Completed bool   `json:"completed"`
			CanClaim  bool   `json:"canClaim"`
		} `json:"quests"`
		Available []struct {
			ID any `json:"id"`
		} `json:"available"`
	}
	if err := ev.Unmarshal(&data); err == nil {
		for _, q := range data.Quests {
			if q.CanClaim || q.Completed || q.Status == "completed" {
				log.Printf("[%s: %s] 📜 [QUEST] Recompensa pronta para coleta (Quest ID: %v)! Reivindicando…", b.config.ID, b.status.CharName, q.ID)
				_ = sock.QuestClaim(q.ID)
			}
		}
		for _, a := range data.Available {
			log.Printf("[%s: %s] 📜 [QUEST] Nova quest diária disponível (Quest ID: %v)! Aceitando…", b.config.ID, b.status.CharName, a.ID)
			_ = sock.QuestPick(a.ID)
		}
	}
}

// handlePreyState selects current hunt monsters or rolls free rerolls
func (b *Bot) handlePreyState(sock *client.SocketClient, ev protocol.RawEvent) {
	var state struct {
		Slots []struct {
			Index      int      `json:"index"`
			State      string   `json:"state"`
			Monsters   []string `json:"monsters"`
			FreeReroll bool     `json:"freeReroll"`
			MonsterID  string   `json:"monsterId"`
			BonusType  string   `json:"bonusType"`
		} `json:"slots"`
	}
	if err := ev.Unmarshal(&state); err == nil {
		b.mu.RLock()
		curHunt := HuntOrder[b.currentHuntIndex]
		if b.status.Hunt != nil {
			curHunt = b.status.Hunt.HuntID
		}
		huntMonsters := GetHuntMonsters(curHunt)
		b.mu.RUnlock()

		targetSet := make(map[string]bool)
		for _, m := range huntMonsters {
			targetSet[m.BestiaryID] = true
		}

		for idx, slot := range state.Slots {
			slotIdx := slot.Index
			if slotIdx == 0 && idx > 0 {
				slotIdx = idx
			}
			if slot.State == "list-selection" || slot.State == "inactive" || slot.State == "empty" {
				picked := false
				for _, mID := range slot.Monsters {
					if targetSet[mID] {
						log.Printf("[%s: %s] 🎯 [PREY] Monstro da caçada atual (%s) encontrado no slot %d! Ativando bônus de XP…",
							b.config.ID, b.status.CharName, mID, slotIdx)
						_ = sock.Send("prey-select-mode", map[string]any{"slot": slotIdx, "mode": "xp"})
						_ = sock.PreyPick(slotIdx, mID)
						picked = true
						break
					}
				}
				if !picked && slot.FreeReroll {
					log.Printf("[%s: %s] 🎯 [PREY] Slot %d sem alvos da hunt atual. Rolando lista gratuita…", b.config.ID, b.status.CharName, slotIdx)
					_ = sock.PreyListReroll(slotIdx)
				}
			}
		}
	}
}

// handleForgeDust automatically converts dust when reaching high capacity
func (b *Bot) handleForgeDust(sock *client.SocketClient, ev protocol.RawEvent) {
	var data struct {
		Dust int `json:"dust"`
		Cap  int `json:"cap"`
	}
	if err := ev.Unmarshal(&data); err == nil {
		if data.Dust >= 100 || (data.Cap > 0 && data.Dust >= data.Cap-20) {
			log.Printf("[%s: %s] ✨ [FORJA] Poeira acumulada (%d/%d). Convertendo em slivers automaticamente…",
				b.config.ID, b.status.CharName, data.Dust, data.Cap)
			_ = sock.ForgeAutoConvert(true)
			_ = sock.ForgeConvert(data.Dust)
		}
	}
}

// handleDailyBossStatus handles leader coordination for daily boss fights
func (b *Bot) handleDailyBossStatus(sock *client.SocketClient, ev protocol.RawEvent) {
	if b.config.Role != "leader" {
		return
	}
	var status struct {
		CanEnter   bool  `json:"canEnter"`
		Available  bool  `json:"available"`
		CooldownMs int64 `json:"cooldownMs"`
		Tier       int   `json:"tier"`
	}
	if err := ev.Unmarshal(&status); err == nil {
		if status.CanEnter || status.Available || (status.CooldownMs <= 0) {
			log.Printf("[%s: %s] 🐉 [DAILY BOSS] Chefe diário disponível! Criando sala de boss da Party…", b.config.ID, b.status.CharName)
			_ = sock.DailyBossCreate(0)
			time.Sleep(500 * time.Millisecond)

			for _, partner := range b.config.PartyMembers {
				if partner != "" {
					log.Printf("[%s: %s] 🐉 [DAILY BOSS] Convidando parceiro %s para o boss…", b.config.ID, b.status.CharName, partner)
					_ = sock.DailyBossInvite(partner)
				}
			}
		}
	}
}

// handleDailyBossInvited lets party followers accept and join boss instance
func (b *Bot) handleDailyBossInvited(sock *client.SocketClient, ev protocol.RawEvent) {
	var data struct {
		RoomID any    `json:"roomId"`
		Leader string `json:"leader"`
	}
	if err := ev.Unmarshal(&data); err == nil {
		log.Printf("[%s: %s] 🐉 [DAILY BOSS] Convite de boss recebido do líder! Entrando na sala e confirmando pronto…", b.config.ID, b.status.CharName)
		_ = sock.DailyBossJoin(data.RoomID)
		time.Sleep(300 * time.Millisecond)
		_ = sock.DailyBossReady(true)
	}
}

// handleDailyBossRooms handles room management and starts fight when team is ready
func (b *Bot) handleDailyBossRooms(sock *client.SocketClient, ev protocol.RawEvent) {
	if b.config.Role != "leader" {
		return
	}
	var room struct {
		Ready bool `json:"ready"`
	}
	if err := ev.Unmarshal(&room); err == nil {
		if room.Ready {
			log.Printf("[%s: %s] 🐉 [DAILY BOSS] Toda a Party está pronta! Iniciando confronto contra o Boss…", b.config.ID, b.status.CharName)
			_ = sock.DailyBossStart()
		}
	}
}
