package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"huntera-bot/pkg/client"
	"huntera-bot/pkg/protocol"
)

type Bot struct {
	config AccountConfig

	mu     sync.RWMutex
	status AccountStatus

	socket *client.SocketClient

	currentHuntIndex int
	retreating       bool
	buyingStamina    bool
	sessionExpired   bool
}

func NewBot(cfg AccountConfig) *Bot {
	huntIdx := 0
	if cfg.HuntID != "" {
		for i, h := range HuntOrder {
			if h == cfg.HuntID {
				huntIdx = i
				break
			}
		}
	}

	return &Bot{
		config:           cfg,
		currentHuntIndex: huntIdx,
		status: AccountStatus{
			ID: cfg.ID,
			Bestiary: &BestiaryState{
				CurrentMonster: "Iniciando…",
				MonsterID:      "rat",
				Required:       2500,
				Stages:         make(map[string]int),
				ByMonster:      make(map[string]int),
			},
			Analyzer:  &AnalyzerState{},
			UpdatedAt: time.Now().UnixMilli(),
		},
	}
}

func (b *Bot) GetStatus() AccountStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Deep copy to prevent race conditions
	cp := b.status
	if b.status.Hunt != nil {
		h := *b.status.Hunt
		cp.Hunt = &h
	}
	if b.status.Bestiary != nil {
		bst := *b.status.Bestiary
		stages := make(map[string]int, len(b.status.Bestiary.Stages))
		for k, v := range b.status.Bestiary.Stages {
			stages[k] = v
		}
		byMonster := make(map[string]int, len(b.status.Bestiary.ByMonster))
		for k, v := range b.status.Bestiary.ByMonster {
			byMonster[k] = v
		}
		bst.Stages = stages
		bst.ByMonster = byMonster
		cp.Bestiary = &bst
	}
	if b.status.Analyzer != nil {
		an := *b.status.Analyzer
		cp.Analyzer = &an
	}
	return cp
}

func (b *Bot) persistStatus() {
	st := b.GetStatus()
	st.UpdatedAt = time.Now().UnixMilli()

	_ = os.MkdirAll("data", 0755)
	filename := filepath.Join("data", fmt.Sprintf("status_%s.json", b.config.ID))
	data, err := json.MarshalIndent(st, "", "  ")
	if err == nil {
		_ = os.WriteFile(filename, data, 0644)
	}
}

func (b *Bot) Run(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				b.persistStatus()
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			if b.socket != nil {
				b.socket.Close()
			}
			return
		default:
		}

		err := b.runSession(ctx)
		if err != nil {
			log.Printf("[%s] Sessão encerrou com erro: %v. Reconectando em 5s…", b.config.ID, err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (b *Bot) runSession(ctx context.Context) error {
	log.Printf("[%s] Autenticando com e-mail: %s…", b.config.ID, b.config.Email)
	sess, err := client.LoginWithCredentials(b.config.Email, b.config.Password)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	chars, err := client.GetCharacters(sess.Cookie)
	if err != nil || len(chars) == 0 {
		return fmt.Errorf("failed to fetch characters: %v", err)
	}

	var targetChar client.Character
	if b.config.CharName != "" {
		for _, c := range chars {
			if c.Name == b.config.CharName {
				targetChar = c
				break
			}
		}
	}
	if targetChar.ID == "" {
		targetChar = chars[0]
	}

	b.mu.Lock()
	b.status.CharName = targetChar.Name
	b.status.Level = targetChar.Level
	b.mu.Unlock()

	ticket, err := client.GetGameTicket(targetChar.ID, sess.Cookie)
	if err != nil {
		return fmt.Errorf("failed to get game ticket: %w", err)
	}

	sock := client.NewSocketClient(sess.Cookie)
	b.socket = sock

	b.setupSocketHandlers(sock)

	err = sock.Connect(targetChar.Name, ticket.Ticket, ticket.WebsocketURL)
	if err != nil {
		return fmt.Errorf("websocket connection failed: %w", err)
	}

	b.mu.Lock()
	b.status.InGame = true
	b.mu.Unlock()
	log.Printf("[%s: %s] Conectado com sucesso ao WebSocket ✓", b.config.ID, targetChar.Name)

	disconnectChan := make(chan struct{})
	sock.On("disconnected", func(_ protocol.RawEvent) {
		b.mu.Lock()
		b.status.InGame = false
		b.mu.Unlock()
		select {
		case <-disconnectChan:
		default:
			close(disconnectChan)
		}
	})

	// Watchdog de caça: se estiver conectado e sem hunt (fora de combate), inicia a hunt atual
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-disconnectChan:
				return
			case <-ticker.C:
				b.mu.RLock()
				inGame := b.status.InGame
				inHunt := b.status.Hunt != nil
				ret := b.retreating
				exp := b.sessionExpired
				huntID := HuntOrder[b.currentHuntIndex]
				b.mu.RUnlock()

				if inGame && !inHunt && !ret && !exp {
					_ = sock.StartHunt(huntID, b.config.Tier)
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
		sock.Close()
		return nil
	case <-disconnectChan:
		return fmt.Errorf("websocket disconnected")
	}
}

func (b *Bot) setupSocketHandlers(sock *client.SocketClient) {
	sock.On("connected", func(_ protocol.RawEvent) {
		time.Sleep(1 * time.Second)
		_ = sock.RequestCyclopedia()

		// Se não estiver em hunt, inicia a hunt configurada
		b.mu.RLock()
		inHunt := b.status.Hunt != nil
		b.mu.RUnlock()

		if !inHunt {
			targetHunt := HuntOrder[b.currentHuntIndex]
			_ = sock.StartHunt(targetHunt, b.config.Tier)
		}
	})

	sock.On("instance-enter", func(ev protocol.RawEvent) {
		var data struct {
			ScenarioID string `json:"scenarioId"`
			InstanceID string `json:"instanceId"`
			Tier       int    `json:"tier"`
		}
		if err := ev.Unmarshal(&data); err == nil && data.ScenarioID != "" && data.ScenarioID != "main-city" {
			b.mu.Lock()
			b.status.Hunt = &HuntState{
				HuntID:     data.ScenarioID,
				Tier:       data.Tier,
				InstanceID: data.InstanceID,
			}
			bestiaryID, monsterName := GetHuntMonster(data.ScenarioID)
			b.status.Bestiary.MonsterID = bestiaryID
			b.status.Bestiary.CurrentMonster = monsterName
			if k, ok := b.status.Bestiary.ByMonster[bestiaryID]; ok {
				b.status.Bestiary.Kills = k
			}
			b.mu.Unlock()
			log.Printf("[%s: %s] Entrou na hunt: %s (%s) [Alvo: %s]", b.config.ID, b.status.CharName, data.ScenarioID, data.InstanceID, monsterName)
		}
	})

	sock.On("hunt-portal-entered", func(ev protocol.RawEvent) {
		var data struct {
			HuntID     string `json:"huntId"`
			ScenarioID string `json:"scenarioId"`
			Tier       int    `json:"tier"`
		}
		if err := ev.Unmarshal(&data); err == nil {
			id := data.HuntID
			if id == "" {
				id = data.ScenarioID
			}
			if id != "" {
				b.mu.Lock()
				b.status.Hunt = &HuntState{HuntID: id, Tier: data.Tier}
				bestiaryID, monsterName := GetHuntMonster(id)
				b.status.Bestiary.MonsterID = bestiaryID
				b.status.Bestiary.CurrentMonster = monsterName
				if k, ok := b.status.Bestiary.ByMonster[bestiaryID]; ok {
					b.status.Bestiary.Kills = k
				}
				b.mu.Unlock()
			}
		}
	})

	sock.On("hunt-leave-pending", func(_ protocol.RawEvent) {
		b.mu.Lock()
		b.status.Hunt = nil
		b.mu.Unlock()
	})

	// Captura de ouro
	sock.On("player-inventory", func(ev protocol.RawEvent) {
		var data struct {
			Gold *int `json:"gold"`
		}
		if err := ev.Unmarshal(&data); err == nil && data.Gold != nil {
			b.mu.Lock()
			b.status.Gold = *data.Gold
			b.mu.Unlock()
		}
	})

	sock.On("inventory-delta", func(ev protocol.RawEvent) {
		var data struct {
			Gold *int `json:"gold"`
		}
		if err := ev.Unmarshal(&data); err == nil && data.Gold != nil {
			b.mu.Lock()
			b.status.Gold = *data.Gold
			b.mu.Unlock()
		}
	})

	// Status do jogador e gerenciamento de stamina
	sock.On("player-stats", func(ev protocol.RawEvent) {
		var data struct {
			Level                  int   `json:"level"`
			Health                 int   `json:"health"`
			MaxHealth              int   `json:"maxHealth"`
			Mana                   int   `json:"mana"`
			MaxMana                int   `json:"maxMana"`
			Experience             int   `json:"experience"`
			ExperienceNeeded       int   `json:"experienceNeeded"`
			HuntSessionRemainingMs int64 `json:"huntSessionRemainingMs"`
			StaminaMs              int64 `json:"staminaMs"`
			StaminaRefillCost      int   `json:"staminaRefillCost"`
			StaminaRefillsLeft     int   `json:"staminaRefillsLeft"`
		}
		if err := ev.Unmarshal(&data); err == nil {
			b.mu.Lock()
			if data.Level > 0 {
				b.status.Level = data.Level
			}
			b.status.HP = data.Health
			b.status.MaxHP = data.MaxHealth
			b.status.Mana = data.Mana
			b.status.MaxMana = data.MaxMana
			if data.Experience > 0 {
				b.status.Experience = data.Experience
			}
			if data.ExperienceNeeded > 0 {
				b.status.ExperienceNeeded = data.ExperienceNeeded
			}
			b.status.SessionRemainingMs = data.HuntSessionRemainingMs
			b.status.StaminaMs = data.StaminaMs
			b.status.StaminaRefillCost = data.StaminaRefillCost
			b.status.StaminaRefillsLeft = data.StaminaRefillsLeft

			gold := b.status.Gold
			cost := data.StaminaRefillCost
			refills := data.StaminaRefillsLeft
			sessionMs := data.HuntSessionRemainingMs
			b.mu.Unlock()

			// Auto Stamina Inteligente
			if sessionMs > 0 && sessionMs <= 120000 {
				if b.config.AutoStamina && gold >= cost && refills > 0 && !b.buyingStamina {
					b.buyingStamina = true
					log.Printf("[%s] Sessão baixa (< 2m). Recarregando stamina por %d gp…", b.config.ID, cost)
					_ = sock.BuyStamina(cost)
					go func() {
						time.Sleep(30 * time.Second)
						b.buyingStamina = false
					}()
				} else if !b.config.AutoStamina && !b.sessionExpired {
					b.sessionExpired = true
					log.Printf("[%s] Sessão esgotada. Indo para Treino Idle gratuito (Zero-Waste)!", b.config.ID)
					_ = sock.LeaveHunt()
					_ = sock.SetIdleTraining(true)
				}
			}
		}
	})

	// Vitals e Zero-Waste: Se vida < 25%, recua da hunt para regenerar passivamente
	sock.On("player-vitals", func(ev protocol.RawEvent) {
		var data struct {
			Health *int `json:"health"`
			Mana   *int `json:"mana"`
		}
		if err := ev.Unmarshal(&data); err == nil {
			b.mu.Lock()
			if data.Health != nil {
				b.status.HP = *data.Health
			}
			if data.Mana != nil {
				b.status.Mana = *data.Mana
			}
			hp := b.status.HP
			maxHp := b.status.MaxHP
			inHunt := b.status.Hunt != nil
			b.mu.Unlock()

			if maxHp > 0 && hp > 0 && float64(hp)/float64(maxHp) < 0.25 && inHunt && !b.retreating {
				b.retreating = true
				log.Printf("[%s] HP < 25%%: Recuando temporariamente da hunt para regeneração de graça (Zero-Waste)!", b.config.ID)
				_ = sock.LeaveHunt()
				go func() {
					time.Sleep(15 * time.Second)
					b.retreating = false
					targetHunt := HuntOrder[b.currentHuntIndex]
					_ = sock.StartHunt(targetHunt, b.config.Tier)
					log.Printf("[%s] HP regenerado. Retomando caça de bestiário em %s.", b.config.ID, targetHunt)
				}()
			}
		}
	})

	// Bestiário
	sock.On("bestiary-progress", func(ev protocol.RawEvent) {
		var data struct {
			BonusPercent  *int           `json:"bonusPercent"`
			Completed     *int           `json:"completed"`
			Total         *int           `json:"total"`
			KillsRequired *int           `json:"killsRequired"`
			Kills         map[string]int `json:"kills"`
			Stages        map[string]int `json:"stages"`
		}
		if err := ev.Unmarshal(&data); err == nil {
			b.mu.Lock()
			bst := b.status.Bestiary

			if data.BonusPercent != nil {
				bst.BonusPercent = *data.BonusPercent
			}
			if data.Completed != nil {
				bst.CompletedMonsters = *data.Completed
			}
			if data.Total != nil {
				bst.TotalMonsters = *data.Total
			}
			if data.KillsRequired != nil {
				bst.Required = *data.KillsRequired
			}

			// Descobrir monstro alvo da hunt atual
			currentHunt := HuntOrder[b.currentHuntIndex]
			if b.status.Hunt != nil {
				currentHunt = b.status.Hunt.HuntID
			}
			bestiaryID, monsterName := GetHuntMonster(currentHunt)
			bst.CurrentMonster = monsterName
			bst.MonsterID = bestiaryID

			if data.Kills != nil {
				for k, v := range data.Kills {
					bst.ByMonster[k] = v
				}
				if k, ok := bst.ByMonster[bestiaryID]; ok {
					bst.Kills = k
				}
				// Total de abates acumulados
				total := 0
				for _, v := range bst.ByMonster {
					total += v
				}
				bst.TotalKills = total
			}

			if data.Stages != nil {
				for k, v := range data.Stages {
					bst.Stages[k] = v
				}
			}

			kills := bst.Kills
			req := bst.Required
			stage := bst.Stages[bestiaryID]
			b.mu.Unlock()

			// Auto unlock de estágio do bestiário
			if kills >= req && req > 0 {
				_ = sock.UnlockBestiary(bestiaryID)
				log.Printf("[%s] ⭐ [BESTIÁRIO] Desbloqueio enviado para %s (+XP Permanente)", b.config.ID, monsterName)
			}

			// Se concluiu o monstro (3 estágios ou 2500 kills), avança para a próxima hunt
			if stage >= 3 || (kills >= 2500 && req >= 2500) {
				log.Printf("[%s] 🏆 [BESTIÁRIO] Monstro %s concluído! Avançando próxima hunt…", b.config.ID, monsterName)
				b.advanceToNextHunt(sock)
			}
		}
	})

	sock.On("hunt-analyzer-update", func(ev protocol.RawEvent) {
		var data struct {
			Kills      int `json:"kills"`
			Experience int `json:"experience"`
			LootValue  int `json:"lootValue"`
			Waste      int `json:"waste"`
		}
		if err := ev.Unmarshal(&data); err == nil {
			b.mu.Lock()
			b.status.Analyzer.Kills = data.Kills
			b.status.Analyzer.Experience = data.Experience
			b.status.Analyzer.LootValue = data.LootValue
			b.status.Analyzer.Waste = data.Waste
			b.mu.Unlock()
		}
	})

	sock.On("experience-gain", func(ev protocol.RawEvent) {
		var data struct {
			Value int `json:"value"`
			Exp   int `json:"exp"`
		}
		if err := ev.Unmarshal(&data); err == nil {
			gain := data.Value
			if gain == 0 {
				gain = data.Exp
			}
			if gain > 0 {
				b.mu.Lock()
				b.status.Analyzer.Experience += gain
				b.status.Analyzer.Kills++
				b.status.Experience += gain
				b.status.Bestiary.Kills++
				b.status.Bestiary.TotalKills++
				if b.status.Bestiary.MonsterID != "" {
					b.status.Bestiary.ByMonster[b.status.Bestiary.MonsterID]++
				}
				b.mu.Unlock()
			}
		}
	})
}

func (b *Bot) advanceToNextHunt(sock *client.SocketClient) {
	if b.currentHuntIndex+1 < len(HuntOrder) {
		b.currentHuntIndex++
		nextHunt := HuntOrder[b.currentHuntIndex]
		bestiaryID, monsterName := GetHuntMonster(nextHunt)
		b.mu.Lock()
		b.status.Bestiary.MonsterID = bestiaryID
		b.status.Bestiary.CurrentMonster = monsterName
		if k, ok := b.status.Bestiary.ByMonster[bestiaryID]; ok {
			b.status.Bestiary.Kills = k
		}
		inHunt := b.status.Hunt != nil
		b.mu.Unlock()
		log.Printf("[%s] 🚀 [BESTIÁRIO] Avançando para: %s (%d/%d) [Alvo: %s]", b.config.ID, nextHunt, b.currentHuntIndex+1, len(HuntOrder), monsterName)
		if inHunt {
			_ = sock.ChangeHunt(nextHunt, b.config.Tier)
		} else {
			_ = sock.StartHunt(nextHunt, b.config.Tier)
		}
	} else {
		log.Printf("[%s] 🎉 TODOS OS BESTIÁRIOS CONCLUÍDOS COM SUCESSO!", b.config.ID)
	}
}
