package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

	lastBestiaryUnlock      time.Time
	lastQuickSell           time.Time
	lastPartyInvite         time.Time
	lastFollowAttempt       time.Time
	lastPaladinEquipAttempt time.Time
	lastStaminaCheck        time.Time
	npcPrices               map[int]int
	auctionPrices           map[int]int
}

func NewBot(cfg AccountConfig) *Bot {
	if len(cfg.PartyMembers) == 0 && cfg.PartnerName != "" {
		for _, p := range strings.Split(cfg.PartnerName, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				cfg.PartyMembers = append(cfg.PartyMembers, p)
			}
		}
	}
	huntIdx := 0
	if cfg.HuntID != "" {
		for i, h := range HuntOrder {
			if h == cfg.HuntID {
				huntIdx = i
				break
			}
		}
	}

	targetStage := cfg.TargetStage
	if targetStage <= 0 {
		targetStage = DefaultTargetStage
	}

	return &Bot{
		config:           cfg,
		currentHuntIndex: huntIdx,
		npcPrices:        make(map[int]int),
		auctionPrices:    make(map[int]int),
		status: AccountStatus{
			ID:           cfg.ID,
			OnlyTraining: cfg.OnlyTraining,
			Bestiary: &BestiaryState{
				CurrentMonster: "Iniciando…",
				MonsterID:      "rat",
				Required:       2500,
				TargetStage:    targetStage,
				Stages:         make(map[string]int),
				ByMonster:      make(map[string]int),
			},
			Analyzer:  &AnalyzerState{},
			UpdatedAt: time.Now().UnixMilli(),
		},
	}
}

func (b *Bot) handleItemProtection(sock *client.SocketClient, item *InventorySlot) {
	if item == nil || item.UID == nil {
		return
	}
	b.mu.RLock()
	aucPrice := b.auctionPrices[item.ItemID]
	npcPrice := b.npcPrices[item.ItemID]
	b.mu.RUnlock()

	// IDs dos produtos de imbuement e itens mais procurados do leilão:
	// 10311: Sabretooth, 11492: Rope Belt, 11444: Protective Charm, 9685: Vampire Teeth, 12517: Medicine Pouch, 5911: Red Piece of Cloth, 3079: Boots of Haste
	isImbuementOrValuable := item.ItemID == 10311 || item.ItemID == 11492 || item.ItemID == 11444 ||
		item.ItemID == 9685 || item.ItemID == 12517 || item.ItemID == 5911 || item.ItemID == 3079

	// Regra de Leilão: produtos valiosos, itens com valor >= 5.000 gp ou com tier > 0 são protegidos contra auto-sell
	if (isImbuementOrValuable || aucPrice >= 5000 || npcPrice >= 5000 || item.Tier > 0) && !item.SellLocked {
		log.Printf("[%s] 💎 [LEILÃO] Item valioso/imbuement detectado (ItemID: %d, Leilão: %d gp, NPC: %d gp, Tier: %d)! Ativando Trava de Venda (set-sell-lock)…",
			b.config.ID, item.ItemID, aucPrice, npcPrice, item.Tier)
		_ = sock.SetSellLock(item.UID, true)
		item.SellLocked = true
	}
}

func (b *Bot) getTargetHuntLocked() (int, string) {
	lvl := b.status.Level
	tStage := b.config.TargetStage
	if tStage <= 0 {
		tStage = DefaultTargetStage
	}
	req := 2500
	if b.status.Bestiary != nil && b.status.Bestiary.Required > 0 {
		req = b.status.Bestiary.Required
	}
	var stages, kills map[string]int
	if b.status.Bestiary != nil {
		stages = b.status.Bestiary.Stages
		kills = b.status.Bestiary.ByMonster
	}

	// Se configurado uma HuntID inicial específica e ela ainda não foi completada no estágio alvo:
	if b.config.HuntID != "" {
		monsters := GetHuntMonsters(b.config.HuntID)
		targetKills := GetRequiredKillsForStage(tStage-1, req)
		incomplete := false
		for _, m := range monsters {
			if stages[m.BestiaryID] < tStage && kills[m.BestiaryID] < targetKills {
				incomplete = true
				break
			}
		}
		if incomplete {
			for i, h := range HuntOrder {
				if h == b.config.HuntID {
					return i, h
				}
			}
		}
	}

	return FindFirstIncompleteHunt(stages, kills, tStage, req, lvl)
}

func (b *Bot) getTargetHunt() (int, string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getTargetHuntLocked()
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

	backoff := 5 * time.Second
	for {
		select {
		case <-ctx.Done():
			if b.socket != nil {
				b.socket.Close()
			}
			return
		default:
		}

		startTime := time.Now()
		err := b.runSession(ctx)
		if err != nil {
			log.Printf("[%s] Sessão encerrou: %v. Próxima tentativa em %v…", b.config.ID, err, backoff)
		}

		// Se a sessão durou mais de 1 minuto, reseta o backoff para 5s
		if time.Since(startTime) > 1*time.Minute {
			backoff = 5 * time.Second
		} else {
			// Se o servidor estiver fora (manutenção prolongada), aumenta o intervalo até 30s
			backoff += 5 * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

func (b *Bot) runSession(ctx context.Context) error {
	b.mu.Lock()
	b.retreating = false
	b.buyingStamina = false
	b.sessionExpired = false
	b.mu.Unlock()

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
	tStage := b.config.TargetStage
	if tStage <= 0 {
		tStage = DefaultTargetStage
	}
	req := 2500
	if b.status.Bestiary != nil && b.status.Bestiary.Required > 0 {
		req = b.status.Bestiary.Required
	}
	var stages, kills map[string]int
	if b.status.Bestiary != nil {
		stages = b.status.Bestiary.Stages
		kills = b.status.Bestiary.ByMonster
	}
	initialIdx, _ := FindFirstIncompleteHunt(stages, kills, tStage, req, targetChar.Level)
	b.currentHuntIndex = initialIdx
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
				staminaMs := b.status.StaminaMs
				ret := b.retreating
				exp := b.sessionExpired
				b.mu.RUnlock()

				// Se o personagem estiver morto (HP <= 0), envia cancel-hunt e revive para retornar ao templo
				b.mu.RLock()
				hp := b.status.HP
				maxHp := b.status.MaxHP
				b.mu.RUnlock()
				if inGame && maxHp > 0 && hp <= 0 {
					log.Printf("[%s: %s] 💀 Personagem detectado com HP 0. Forçando cancel-hunt e executando revive…", b.config.ID, b.status.CharName)
					_ = sock.LeaveHunt()
					_ = sock.Send("cancel-hunt", map[string]any{})
					time.Sleep(500 * time.Millisecond)
					_ = sock.Send("revive", map[string]any{})
					time.Sleep(1 * time.Second)
					_ = sock.BlessingsOpen()
					time.Sleep(300 * time.Millisecond)
					_ = sock.BuyBlessing("all")
				}

				// Se configurado exclusivamente para treino idle (acc3 e acc4 aguardando acc1 e acc2 alcançarem level):
				if inGame && b.config.OnlyTraining && hp > 0 {
					if inHunt {
						log.Printf("[%s: %s] 🏋️ Modo Apenas Treino ativo. Saindo de caçada para focar 100%% em Treino Idle!", b.config.ID, b.status.CharName)
						b.mu.Lock()
						b.status.Hunt = nil
						b.mu.Unlock()
						_ = sock.LeaveHunt()
						time.Sleep(500 * time.Millisecond)
					}
					_ = sock.SetIdleTraining(true)
					continue
				}

				// 1. Coordenação Coletiva de Stamina (Regra: assim que a stamina acabar de um, todos devem ir treinar):
				shouldRest, lowChar, lowMs := GlobalCoordinator.ShouldTeamRest()
				if shouldRest {
					if !GlobalCoordinator.IsTeamResting() {
						GlobalCoordinator.SetTeamResting(true, fmt.Sprintf("%s com %d ms (< 5 min)", lowChar, lowMs))
						log.Printf("🚨 [COORDENADOR 4-VOC] Stamina de %s esgotou/baixa (< 5 min: %d ms)! ORDEM PARA O TIME INTEIRO: SAIR DA HUNT E ATIVAR TREINO IDLE!", lowChar, lowMs)
					}
				} else if GlobalCoordinator.IsTeamResting() {
					// Time em descanso: verifica se todos recuperaram pelo menos 10 minutos (600.000 ms) para caçar com consistência
					if canResume, _ := GlobalCoordinator.CanTeamResume(600000); canResume {
						GlobalCoordinator.SetTeamResting(false, "")
						log.Printf("✨ [COORDENADOR 4-VOC] Todas as 4 contas recuperaram stamina suficiente (>= 10 min)! ORDEM PARA O TIME: RETOMAR HUNT COMPARTILHADA!")
					}
				}

				// Se o time estiver em descanso coletivo:
				if GlobalCoordinator.IsTeamResting() {
					if inHunt {
						log.Printf("[%s: %s] 🏋️ [TEAM REST] Pausando caçada e ativando Treino Idle em sincronia com o time!", b.config.ID, b.status.CharName)
						b.mu.Lock()
						b.status.Hunt = nil
						b.sessionExpired = true
						b.mu.Unlock()
						_ = sock.LeaveHunt()
						time.Sleep(300 * time.Millisecond)
						_ = sock.SetIdleTraining(true)
					} else if !b.sessionExpired {
						b.sessionExpired = true
						_ = sock.SetIdleTraining(true)
					}

					// Verificação periódica de recuperação de stamina a cada 2 minutos
					now := time.Now()
					if now.Sub(b.lastStaminaCheck) > 2*time.Minute {
						b.lastStaminaCheck = now
						_ = sock.SetIdleTraining(false)
						time.Sleep(200 * time.Millisecond)
						// Envia ping/consulta para atualizar status
						_ = sock.SetIdleTraining(true)
					}
					continue
				}

				// Se liberado para caçar e com stamina suficiente (>= 5 min):
				if inGame && !inHunt && staminaMs >= 300000 && !ret && !exp && hp > 0 && !b.config.OnlyTraining {
					b.mu.Lock()
					b.status.Hunt = nil
					idx, targetHunt := b.getTargetHuntLocked()
					b.currentHuntIndex = idx
					b.mu.Unlock()

					if b.config.Role == "leader" {
						_ = sock.SetIdleTraining(false)
						time.Sleep(200 * time.Millisecond)
						log.Printf("[%s: %s] ⚔️ [LÍDER 4-VOC] Iniciando Hunt em equipe: %s (tier %d)…", b.config.ID, b.status.CharName, targetHunt, b.config.Tier)
						_ = sock.StartHunt(targetHunt, b.config.Tier)
					} else {
						// Follower: NÃO inicia caçada solo! Segue o líder na sala da party!
						_ = sock.SetIdleTraining(false)
						time.Sleep(200 * time.Millisecond)
						log.Printf("[%s: %s] ⚔️ [FOLLOWER 4-VOC] Seguindo líder para a sala da hunt…", b.config.ID, b.status.CharName)
						_ = sock.PartyFollowLeader()
					}
				}

				// Se for o Líder da Party, envia convite periódico para todos os membros:
				if inGame && b.config.Role == "leader" && len(b.config.PartyMembers) > 0 {
					now := time.Now()
					if now.Sub(b.lastPartyInvite) > 12*time.Second {
						b.lastPartyInvite = now
						for _, member := range b.config.PartyMembers {
							_ = sock.PartyInviteName(member)
							time.Sleep(150 * time.Millisecond)
						}
					}
				}

				// Se for Follower fora da hunt (aguardando o líder puxar ou entrar na sala):
				if inGame && b.config.Role == "follower" && !inHunt && hp > 0 && !b.config.OnlyTraining && !GlobalCoordinator.IsTeamResting() {
					now := time.Now()
					if now.Sub(b.lastFollowAttempt) > 6*time.Second {
						b.lastFollowAttempt = now
						_ = sock.SetIdleTraining(false)
						_ = sock.PartyFollowLeader()
					}
				}

				// Verificação e Desbloqueio Automático de Estágio do Bestiário no Watchdog:
				if inGame && inHunt {
					b.mu.Lock()
					bst := b.status.Bestiary
					currentHunt := HuntOrder[b.currentHuntIndex]
					if b.status.Hunt != nil {
						currentHunt = b.status.Hunt.HuntID
					}
					tStage := b.config.TargetStage
					if tStage <= 0 {
						tStage = DefaultTargetStage
					}
					baseReq := 2500
					huntMonsters := GetHuntMonsters(currentHunt)
					b.mu.Unlock()

					if bst != nil {
						for _, m := range huntMonsters {
							b.mu.RLock()
							k := bst.ByMonster[m.BestiaryID]
							stage := bst.Stages[m.BestiaryID]
							gold := b.status.Gold
							b.mu.RUnlock()

							req := GetRequiredKillsForStage(stage, baseReq)
							// Regra Zero-Gasto: Só tenta desbloquear se stage < tStage. Se stage >= 1 (custo de 30k+ gp),
							// só tenta se o char tiver bastante ouro livre (>= 50k gp):
							if k >= req && stage < tStage && (stage == 0 || gold >= 50000) {
								now := time.Now()
								if now.Sub(b.lastBestiaryUnlock) > 15*time.Second {
									b.lastBestiaryUnlock = now
									_ = sock.UnlockBestiary(m.BestiaryID)
									_ = sock.RequestCyclopedia()

									b.mu.Lock()
									bst.Stages[m.BestiaryID] = stage + 1
									if bst.MonsterID == m.BestiaryID {
										bst.Stage = stage + 1
										bst.Required = GetRequiredKillsForStage(stage+1, baseReq)
									}
									b.mu.Unlock()

									rewardType := "+1% XP Permanente"
									if (stage+1)%4 == 0 {
										rewardType = "⚔️ +1% DANO TOTAL Permanente"
									}
									log.Printf("[%s] ⭐ [BESTIÁRIO] Desbloqueio enviado para %s [Estágio %d/%d] (%d/%d kills) [%s]",
										b.config.ID, m.Name, stage+1, tStage, k, req, rewardType)
								}
							}
						}
					}
				}
			}
		}
	}()

	// Sincronização periódica da Cyclopedia (a cada 60 segundos) para atualizar Bestiário
	go func() {
		ticker := time.NewTicker(60 * time.Second)
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
				b.mu.RUnlock()
				if inGame {
					_ = sock.RequestCyclopedia()
				}
			}
		}
	}()

	// Quick-Sell Periódico: despacha loot automaticamente a cada 30 minutos
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
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
				b.mu.RUnlock()

				if inGame && inHunt {
					log.Printf("[%s] 📦 [AUTO-SELL] Despachando loot via Quick-Sell (courier)…", b.config.ID)
					_ = sock.HuntQuickSell()
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
	b.setupPartyFeatures(sock)

	sock.On("connected", func(_ protocol.RawEvent) {
		time.Sleep(1 * time.Second)
		_ = sock.RequestCyclopedia()

		// 🛡️ Garante blessings ao conectar (grátis até o nível limite ou compradas)
		_ = sock.BlessingsOpen()
		time.Sleep(300 * time.Millisecond)
		_ = sock.BuyBlessing("all")

		// Regra de auto-sell oficial: vende na capacidade cheia e a cada 30 min
		_ = sock.SetHuntSellRules(true, true)
		time.Sleep(500 * time.Millisecond)
		_ = sock.HuntQuickSell()
		_ = sock.Send("dps-breakdown-watch", map[string]any{})
		_ = sock.Send("depot-open", map[string]any{})
		_ = sock.Send("shop-open", map[string]any{})

		// ⚔️ Action Bar / Magias de Combate por vocação
		b.initActionBar(sock)

		// 🎁 Recompensa Diária (Claim Daily Reward)
		b.claimDailyReward(sock)

		// ⚒️ Forja Automática de Dust (Converter poeira acumulada em itens/upgrades)
		_ = sock.Send("forge-open", map[string]any{})
		time.Sleep(300 * time.Millisecond)
		_ = sock.Send("forge-auto-convert", map[string]any{})

		// 🐉 Daily Boss (Consulta status de chefe diário)
		_ = sock.Send("daily-boss-open", map[string]any{})

		// 🔮 Imbuements (Abre santuário de imbuements)
		_ = sock.Send("imbuement-open", map[string]any{})

		// 🛡️ Guildas (Consulta diretório de guildas para bônus de XP passivo)
		_ = sock.Send("guild-directory-request", map[string]any{})

		// Se configurado exclusivamente para treino idle (acc3 e acc4 aguardando acc1 e acc2):
		if b.config.OnlyTraining {
			_ = sock.LeaveHunt()
			time.Sleep(500 * time.Millisecond)
			_ = sock.SetIdleTraining(true)
			log.Printf("[%s: %s] 🏋️ Modo Apenas Treino ativo! Focando em treinar skills até que a Duo 1 alcance o level.", b.config.ID, b.status.CharName)
			return
		}

		b.mu.Lock()
		inHunt := b.status.Hunt != nil
		staminaMs := b.status.StaminaMs
		idx, targetHunt := b.getTargetHuntLocked()
		b.currentHuntIndex = idx
		b.mu.Unlock()

		if !inHunt && staminaMs >= 300000 {
			if b.config.Role == "follower" {
				_ = sock.PartyFollowLeader()
				time.Sleep(500 * time.Millisecond)
			}
			_ = sock.StartHunt(targetHunt, b.config.Tier)
		}
	})

	sock.On("instance-enter", func(ev protocol.RawEvent) {
		var data struct {
			ScenarioID string `json:"scenarioId"`
			InstanceID string `json:"instanceId"`
			Tier       int    `json:"tier"`
		}
		if err := ev.Unmarshal(&data); err == nil {
			if data.ScenarioID == "" || data.ScenarioID == "main-city" {
				b.mu.Lock()
				b.status.Hunt = nil
				b.mu.Unlock()
				log.Printf("[%s: %s] Na cidade principal (%s). Sala de caça liberada.", b.config.ID, b.status.CharName, data.ScenarioID)

				if b.config.ID == "acc1" && !b.config.OnlyTraining {
					b.handlePaladinCitySupplies(sock)
				}
			} else {
				b.mu.Lock()
				b.status.Hunt = &HuntState{
					HuntID:     data.ScenarioID,
					Tier:       data.Tier,
					InstanceID: data.InstanceID,
				}
				tStage := b.config.TargetStage
				if tStage <= 0 {
					tStage = DefaultTargetStage
				}
				req := 2500
				if b.status.Bestiary != nil && b.status.Bestiary.Required > 0 {
					req = b.status.Bestiary.Required
				}
				bestiaryID, monsterName := GetHuntTargetMonster(data.ScenarioID, b.status.Bestiary.Stages, b.status.Bestiary.ByMonster, tStage, req)
				b.status.Bestiary.MonsterID = bestiaryID
				b.status.Bestiary.CurrentMonster = monsterName
				b.status.Bestiary.TargetStage = tStage
				if k, ok := b.status.Bestiary.ByMonster[bestiaryID]; ok {
					b.status.Bestiary.Kills = k
				}
				b.sessionExpired = false
				b.mu.Unlock()
				log.Printf("[%s: %s] Entrou na hunt: %s (%s) [Alvo: %s (Estágio %d)]", b.config.ID, b.status.CharName, data.ScenarioID, data.InstanceID, monsterName, tStage)
			}
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
				tStage := b.config.TargetStage
				if tStage <= 0 {
					tStage = DefaultTargetStage
				}
				req := 2500
				if b.status.Bestiary != nil && b.status.Bestiary.Required > 0 {
					req = b.status.Bestiary.Required
				}
				bestiaryID, monsterName := GetHuntTargetMonster(id, b.status.Bestiary.Stages, b.status.Bestiary.ByMonster, tStage, req)
				b.status.Bestiary.MonsterID = bestiaryID
				b.status.Bestiary.CurrentMonster = monsterName
				b.status.Bestiary.TargetStage = tStage
				if k, ok := b.status.Bestiary.ByMonster[bestiaryID]; ok {
					b.status.Bestiary.Kills = k
				}
				b.sessionExpired = false
				b.mu.Unlock()
			}
		}
	})

	sock.On("hunt-leave-pending", func(_ protocol.RawEvent) {
		b.mu.Lock()
		b.status.Hunt = nil
		b.mu.Unlock()
	})

	// 🤝 PARTY & HUNT COMPARTILHADA (+100% XP 4 Vocações e Divisão de Custos):
	// Confirmação de convite enviado:
	sock.On("party-invite-confirm", func(ev protocol.RawEvent) {
		log.Printf("[%s: %s] 🤝 [PARTY] Confirmação de convite recebida: %s", b.config.ID, b.status.CharName, string(ev.Payload))
	})

	// Se for convidado para party, aceita imediatamente:
	sock.On("party-invited", func(ev protocol.RawEvent) {
		log.Printf("[%s: %s] 🤝 Convite de Party recebido! (%s) Aceitando…", b.config.ID, b.status.CharName, string(ev.Payload))
		_ = sock.PartyRespond(true)
	})

	// Se a hunt for de equipe ou o líder puxar para a sala, aceita entrar junto:
	sock.On("hunt-team-invited", func(ev protocol.RawEvent) {
		log.Printf("[%s: %s] ⚔️ Convite para entrar na Hunt em equipe recebido! (%s) Aceitando…", b.config.ID, b.status.CharName, string(ev.Payload))
		_ = sock.HuntTeamRespond(true)
		_ = sock.PartyFollowLeader()
	})

	sock.On("hunt-leader-entered", func(ev protocol.RawEvent) {
		log.Printf("[%s: %s] ⚔️ Líder entrou na Hunt! Entrando na sala em equipe…", b.config.ID, b.status.CharName)
		_ = sock.Send("hunt-leader-respond", map[string]any{"accept": true})
		_ = sock.PartyFollowLeader()
	})

	// Oferta de divisão de custos da Party (Party Hunt Splitter):
	sock.On("party-costs-offered", func(_ protocol.RawEvent) {
		log.Printf("[%s: %s] 💰 Oferta de divisão de custos da Party recebida! Aceitando para dividir gastos/lucro…", b.config.ID, b.status.CharName)
		_ = sock.PartyCostsRespond(true)
	})

	// Quando a party atualiza, se for o líder, oferece divisão de custos:
	sock.On("party-update", func(ev protocol.RawEvent) {
		log.Printf("[%s: %s] 🤝 [PARTY] Status da Party atualizado: %s", b.config.ID, b.status.CharName, string(ev.Payload))
		if b.config.Role == "leader" {
			_ = sock.PartyCostsOffer()
		}
	})

	// Mensagens do sistema do jogo (avisos, regras de party, etc.)
	sock.On("system-message", func(ev protocol.RawEvent) {
		log.Printf("[%s: %s] ℹ️ [SISTEMA] Mensagem: %s", b.config.ID, b.status.CharName, string(ev.Payload))
		var msg struct {
			Text string `json:"text"`
		}
		if err := ev.Unmarshal(&msg); err == nil {
			if strings.Contains(msg.Text, "You need at least 5 minutes of stamina") {
				b.sessionExpired = true
				log.Printf("[%s: %s] ⏳ Servidor indicou que ainda precisa recuperar stamina. Mantendo Treino Idle…", b.config.ID, b.status.CharName)
				_ = sock.SetIdleTraining(true)
			}
			if strings.Contains(msg.Text, "invited you to join") {
				log.Printf("[%s: %s] 🤝 Convite de Party detectado na mensagem! Aceitando…", b.config.ID, b.status.CharName)
				_ = sock.PartyRespond(true)
			}
		}
	})

	// Tabela oficial de preços de itens (NPC e Leilão)
	sock.On("item-values", func(ev protocol.RawEvent) {
		var data struct {
			NPC     [][2]int `json:"npc"`
			Auction [][2]int `json:"auction"`
		}
		if err := ev.Unmarshal(&data); err == nil {
			b.mu.Lock()
			for _, pair := range data.NPC {
				b.npcPrices[pair[0]] = pair[1]
			}
			for _, pair := range data.Auction {
				b.auctionPrices[pair[0]] = pair[1]
			}
			b.mu.Unlock()
		}
	})

	// Overflow de Capacidade: dispara venda imediata
	sock.On("capacity-overflow", func(_ protocol.RawEvent) {
		log.Printf("[%s] ⚠️ Capacidade da mochila esgotada! Despachando Quick-Sell de emergência…", b.config.ID)
		_ = sock.HuntQuickSell()
	})

	// Captura de inventário, ouro e proteção de itens valiosos para o leilão
	sock.On("player-inventory", func(ev protocol.RawEvent) {
		_ = os.MkdirAll("data", 0755)
		_ = os.WriteFile(filepath.Join("data", fmt.Sprintf("inventory_%s.json", b.config.ID)), ev.Payload, 0644)
		log.Printf("[%s: %s] 🎒 [INVENTÁRIO] Dados recebidos (%d bytes): %s", b.config.ID, b.status.CharName, len(ev.Payload), string(ev.Payload))

		var rawInv struct {
			Gold      *int                      `json:"gold"`
			Equipment map[string]*InventorySlot `json:"equipment"`
			Slots     []*struct {
				UID         any    `json:"uid"`
				ItemID      int    `json:"itemId"`
				Name        string `json:"name"`
				Slot        string `json:"slot"`
				Attack      int    `json:"attack"`
				Defense     int    `json:"defense"`
				AttackSkill string `json:"attackSkill"`
				Count       int    `json:"count"`
				SellLocked  bool   `json:"sellLocked"`
				Tier        int    `json:"tier"`
			} `json:"slots"`
		}
		if err := ev.Unmarshal(&rawInv); err == nil {
			if rawInv.Gold != nil {
				b.mu.Lock()
				b.status.Gold = *rawInv.Gold
				b.mu.Unlock()
			}
			// Auto-Equip itens para slots desarmados/vazios ou upgrade de arma para Paladino:
			if rawInv.Equipment != nil && !b.config.OnlyTraining {
				isPaladin := b.config.ID == "acc1"

				for _, slotName := range []string{"weapon", "shield", "legs", "helmet", "armor", "boots", "amulet", "ring", "ammo"} {
					curEquip := rawInv.Equipment[slotName]
					needEquip := curEquip == nil

					// Caso especial do Paladino: se slot de arma estiver vazio ou com arma que não é de distância, verifica se há arco/lança na mochila
					if isPaladin && slotName == "weapon" {
						for _, item := range rawInv.Slots {
							if item != nil && item.Slot == "weapon" && item.AttackSkill == "distance" {
								needEquip = true
								break
							}
						}
					}

					if needEquip {
						type invItem = struct {
							UID         any    `json:"uid"`
							ItemID      int    `json:"itemId"`
							Name        string `json:"name"`
							Slot        string `json:"slot"`
							Attack      int    `json:"attack"`
							Defense     int    `json:"defense"`
							AttackSkill string `json:"attackSkill"`
							Count       int    `json:"count"`
							SellLocked  bool   `json:"sellLocked"`
							Tier        int    `json:"tier"`
						}
						var bestItem *invItem
						bestIdx := -1
						for idx, item := range rawInv.Slots {
							if item != nil && item.Slot == slotName {
								if isPaladin && slotName == "weapon" {
									// Paladino prioriza absolutamente armas de distância (bow/spear):
									if item.AttackSkill == "distance" {
										if bestItem == nil || bestItem.AttackSkill != "distance" || item.Attack > bestItem.Attack {
											bestItem = item
											bestIdx = idx
										}
									} else if bestItem == nil {
										bestItem = item
										bestIdx = idx
									}
								} else {
									if bestItem == nil || item.Attack > bestItem.Attack || item.Defense > bestItem.Defense {
										bestItem = item
										bestIdx = idx
									}
								}
							}
						}
						if bestItem != nil && bestIdx >= 0 {
							log.Printf("[%s: %s] 🛡️ [AUTO-EQUIP] Equipando %s no slot '%s' (Atk: %d, Def: %d, Skill: %s, Backpack Index: %d)…",
								b.config.ID, b.status.CharName, bestItem.Name, slotName, bestItem.Attack, bestItem.Defense, bestItem.AttackSkill, bestIdx)
							_ = sock.Send("equip-item", map[string]any{
								"ref": map[string]any{
									"container": "backpack",
									"index":     bestIdx,
								},
							})
							time.Sleep(200 * time.Millisecond)
						}
					}
				}

				// Paladino: garantir que equipe lança/arco da mochila e regras de munição corretas:
				if isPaladin {
					curWeap := rawInv.Equipment["weapon"]
					var slots []*InventorySlot
					for _, s := range rawInv.Slots {
						if s != nil {
							slots = append(slots, &InventorySlot{
								UID:         s.UID,
								ItemID:      s.ItemID,
								Name:        s.Name,
								Slot:        s.Slot,
								Attack:      s.Attack,
								Defense:     s.Defense,
								AttackSkill: s.AttackSkill,
								Count:       s.Count,
								SellLocked:  s.SellLocked,
								Tier:        s.Tier,
							})
						}
					}
					var curSlot *InventorySlot
					if curWeap != nil {
						curSlot = &InventorySlot{
							UID:         curWeap.UID,
							ItemID:      curWeap.ItemID,
							Name:        curWeap.Name,
							Slot:        curWeap.Slot,
							Attack:      curWeap.Attack,
							Defense:     curWeap.Defense,
							AttackSkill: curWeap.AttackSkill,
							Count:       curWeap.Count,
						}
					}
					b.handlePaladinEquip(sock, curSlot, slots)
					b.setupPaladinAmmoRules(sock)
				}
			}

			// Proteção de itens valiosos e Auto-Market
			var allSlots []*InventorySlot
			for _, slot := range rawInv.Slots {
				if slot != nil {
					slotObj := &InventorySlot{
						UID:        slot.UID,
						ItemID:     slot.ItemID,
						Name:       slot.Name,
						Count:      slot.Count,
						SellLocked: slot.SellLocked,
						Tier:       slot.Tier,
					}
					b.handleItemProtection(sock, slotObj)
					allSlots = append(allSlots, slotObj)
				}
			}
			b.handleMarketBatchSell(sock, allSlots)
		}
	})

	sock.On("depot-update", func(ev protocol.RawEvent) {
		_ = os.MkdirAll("data", 0755)
		_ = os.WriteFile(filepath.Join("data", fmt.Sprintf("depot_%s.json", b.config.ID)), ev.Payload, 0644)
		log.Printf("[%s: %s] 📦 [DEPOT] Conteúdo recebido (%d bytes)", b.config.ID, b.status.CharName, len(ev.Payload))
	})

	sock.On("shop-offers", func(ev protocol.RawEvent) {
		_ = os.MkdirAll("data", 0755)
		_ = os.WriteFile(filepath.Join("data", fmt.Sprintf("shop_offers_%s.json", b.config.ID)), ev.Payload, 0644)
		log.Printf("[%s: %s] 🛒 [SHOP] Ofertas recebidas (%d bytes)", b.config.ID, b.status.CharName, len(ev.Payload))
	})

	sock.On("dps-breakdown", func(ev protocol.RawEvent) {
		_ = os.MkdirAll("data", 0755)
		_ = os.WriteFile(filepath.Join("data", fmt.Sprintf("dps_%s.json", b.config.ID)), ev.Payload, 0644)
		log.Printf("[%s: %s] ⚔️ [DPS-BREAKDOWN] %s", b.config.ID, b.status.CharName, string(ev.Payload))
	})

	sock.On("ammo-selection", func(ev protocol.RawEvent) {
		_ = os.MkdirAll("data", 0755)
		_ = os.WriteFile(filepath.Join("data", fmt.Sprintf("ammo_selection_%s.json", b.config.ID)), ev.Payload, 0644)
		log.Printf("[%s: %s] 🏹 [AMMO-SELECTION] %s", b.config.ID, b.status.CharName, string(ev.Payload))
	})

	sock.On("ammo-rules", func(ev protocol.RawEvent) {
		_ = os.MkdirAll("data", 0755)
		_ = os.WriteFile(filepath.Join("data", fmt.Sprintf("ammo_rules_%s.json", b.config.ID)), ev.Payload, 0644)
		log.Printf("[%s: %s] 🏹 [AMMO-RULES] %s", b.config.ID, b.status.CharName, string(ev.Payload))
	})

	// 🎁 Recompensa Diária Coletada
	sock.On("daily-reward-claimed", func(ev protocol.RawEvent) {
		log.Printf("[%s: %s] 🎁 [RECOMPENSA DIÁRIA] Coletada com sucesso: %s", b.config.ID, b.status.CharName, string(ev.Payload))
	})

	// 🔮 Imbuements Shrine Data
	sock.On("imbuement-shrine", func(ev protocol.RawEvent) {
		_ = os.MkdirAll("data", 0755)
		_ = os.WriteFile(filepath.Join("data", fmt.Sprintf("imbuement_%s.json", b.config.ID)), ev.Payload, 0644)
		log.Printf("[%s: %s] 🔮 [IMBUEMENT SHRINE] Dados recebidos: %s", b.config.ID, b.status.CharName, string(ev.Payload))
	})

	// 🛡️ Guild Directory & Auto-Apply
	sock.On("guild-directory", func(ev protocol.RawEvent) {
		_ = os.MkdirAll("data", 0755)
		_ = os.WriteFile(filepath.Join("data", fmt.Sprintf("guilds_%s.json", b.config.ID)), ev.Payload, 0644)
		log.Printf("[%s: %s] 🛡️ [GUILD DIRECTORY] Lista de guildas recebida (%d bytes)", b.config.ID, b.status.CharName, len(ev.Payload))

		var dir struct {
			Guilds []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Members     int    `json:"members"`
				Recruiting  bool   `json:"recruiting"`
				LevelReq    int    `json:"levelReq"`
				AutoAccept  bool   `json:"autoAccept"`
			} `json:"guilds"`
		}
		if err := ev.Unmarshal(&dir); err == nil {
			for _, g := range dir.Guilds {
				if (g.Recruiting || g.AutoAccept) && (g.LevelReq == 0 || b.status.Level >= g.LevelReq) {
					log.Printf("[%s: %s] 🛡️ [GUILDA] Enviando aplicação para a guilda '%s' (ID: %s) para bônus de XP passivo!",
						b.config.ID, b.status.CharName, g.Name, g.ID)
					_ = sock.Send("guild-apply", map[string]any{"guildId": g.ID})
					break
				}
			}
		}
	})

	sock.On("inventory-delta", func(ev protocol.RawEvent) {
		_ = os.MkdirAll("data", 0755)
		_ = os.WriteFile(filepath.Join("data", fmt.Sprintf("inventory_delta_%s.json", b.config.ID)), ev.Payload, 0644)
		var data struct {
			Gold    *int `json:"gold"`
			Changes []struct {
				Container string `json:"container"`
				Index     int    `json:"index"`
				Slot      string `json:"slot"`
				Item      *struct {
					UID         any    `json:"uid"`
					ItemID      int    `json:"itemId"`
					Name        string `json:"name"`
					Slot        string `json:"slot"`
					Attack      int    `json:"attack"`
					Defense     int    `json:"defense"`
					AttackSkill string `json:"attackSkill"`
					Count       int    `json:"count"`
					SellLocked  bool   `json:"sellLocked"`
					Tier        int    `json:"tier"`
				} `json:"item"`
			} `json:"changes"`
		}
		if err := ev.Unmarshal(&data); err == nil {
			if data.Gold != nil {
				b.mu.Lock()
				b.status.Gold = *data.Gold
				b.mu.Unlock()
			}
			for _, ch := range data.Changes {
				if ch.Item != nil {
					b.handleItemProtection(sock, &InventorySlot{
						UID:        ch.Item.UID,
						ItemID:     ch.Item.ItemID,
						Count:      ch.Item.Count,
						SellLocked: ch.Item.SellLocked,
						Tier:       ch.Item.Tier,
					})
				}
			}
		}
	})

	// Status do jogador e gerenciamento de stamina
	sock.On("player-stats", func(ev protocol.RawEvent) {
		_ = os.MkdirAll("data", 0755)
		_ = os.WriteFile(filepath.Join("data", fmt.Sprintf("stats_%s.json", b.config.ID)), ev.Payload, 0644)
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
			Combat                 struct {
				AttackSkill  string `json:"attackSkill"`
				AttackMax    int    `json:"attackMax"`
				WeaponAttack int    `json:"weaponAttack"`
			} `json:"combat"`
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
			staminaMs := data.StaminaMs
			b.mu.Unlock()

			// Se o Paladino estiver por acaso sem arma de distância equipada, solicita sync de inventário:
			if b.config.ID == "acc1" && !b.config.OnlyTraining && data.Combat.AttackSkill != "distance" {
				if time.Since(b.lastPaladinEquipAttempt) > 60*time.Second {
					b.lastPaladinEquipAttempt = time.Now()
					log.Printf("[%s: %s] 🏹 [PALADINO] Verificando armas de distância no inventário…", b.config.ID, b.status.CharName)
					_ = sock.Send("depot-open", map[string]any{})
				}
			}

			// 1. Se a stamina foi restaurada (Server Save ou descanso >= 5 min) e estávamos em treino idle:
			if staminaMs >= 300000 && b.sessionExpired && !b.config.OnlyTraining {
				b.sessionExpired = false
				log.Printf("[%s] Stamina restaurada (%d ms). Saindo do treino e retomando caça!", b.config.ID, staminaMs)
				_ = sock.SetIdleTraining(false)
				time.Sleep(1 * time.Second)
				if b.config.Role == "follower" {
					_ = sock.PartyFollowLeader()
					time.Sleep(500 * time.Millisecond)
				}
				idx, targetHunt := b.getTargetHunt()
				b.mu.Lock()
				b.currentHuntIndex = idx
				b.mu.Unlock()
				_ = sock.StartHunt(targetHunt, b.config.Tier)
			}

			// 2. Auto Refill Diário: se stamina estiver baixa (< 2h) e houver refills e ouro:
			if staminaMs > 0 && staminaMs < 2*3600*1000 {
				if b.config.AutoStamina && gold >= cost && refills > 0 && !b.buyingStamina {
					b.buyingStamina = true
					log.Printf("[%s] Stamina baixa (< 2h). Comprando refill diário (%d/6) por %d gp…", b.config.ID, refills, cost)
					_ = sock.BuyStamina(cost)
					go func() {
						time.Sleep(30 * time.Second)
						b.buyingStamina = false
					}()
				}
			}

			// 3. Esgotamento de Stamina do dia: se stamina zerar (ou < 5 min) e não for possível recarregar (sem ouro ou sem refills) -> Treino Idle gratuito (Zero-Waste)
			cannotRefill := (!b.config.AutoStamina || gold < cost || refills == 0)
			if staminaMs < 300000 && cannotRefill && !b.sessionExpired {
				b.sessionExpired = true
				log.Printf("[%s: %s] 🏋️ Stamina esgotada/baixa (< 5 min, %d ms, ouro: %d gp, custo: %d gp, refills: %d). Ativando Treino Idle para treinar skills até recuperar!",
					b.config.ID, b.status.CharName, staminaMs, gold, cost, refills)
				_ = sock.LeaveHunt()
				time.Sleep(500 * time.Millisecond)
				_ = sock.SetIdleTraining(true)
			}

			// 4. Renovação automática da sala de caça (Hunt Session 4h) quando a sessão expirar (<= 15s ou já 0) com stamina disponível:
			if sessionMs <= 15000 && staminaMs >= 300000 && !b.sessionExpired && !b.retreating && !b.config.OnlyTraining {
				if b.config.Role != "follower" {
					b.mu.Lock()
					b.status.Hunt = nil
					idx, targetHunt := b.getTargetHuntLocked()
					b.currentHuntIndex = idx
					b.mu.Unlock()
					log.Printf("[%s] 🔄 Sessão da caverna finalizada/zerada (restante: %d ms, stamina: %d ms). Renovando sala em %s…", b.config.ID, sessionMs, staminaMs, targetHunt)
					_ = sock.LeaveHunt()
					time.Sleep(500 * time.Millisecond)
					_ = sock.StartHunt(targetHunt, b.config.Tier)
				}
			}
		}
	})

	// Status de blessings recebido do servidor:
	sock.On("blessings-status", func(ev protocol.RawEvent) {
		var data struct {
			Cost           *int  `json:"cost"`
			FreeUntilLevel *int  `json:"freeUntilLevel"`
			Count          *int  `json:"count"`
			Total          *int  `json:"total"`
			HasAll         *bool `json:"hasAll"`
		}
		if err := ev.Unmarshal(&data); err == nil {
			cost := 0
			if data.Cost != nil {
				cost = *data.Cost
			}
			freeLvl := 0
			if data.FreeUntilLevel != nil {
				freeLvl = *data.FreeUntilLevel
			}
			hasAll := false
			if data.HasAll != nil {
				hasAll = *data.HasAll
			}
			cnt := 0
			if data.Count != nil {
				cnt = *data.Count
			}
			tot := 0
			if data.Total != nil {
				tot = *data.Total
			}
			log.Printf("[%s: %s] 🛡️ [BLESSINGS] Custo: %d gp (grátis até Lv %d). Raw: %s", b.config.ID, b.status.CharName, cost, freeLvl, string(ev.Payload))
			if !hasAll && (tot > 0 && cnt < tot) {
				log.Printf("[%s: %s] 🛡️ [BLESSINGS] Faltando bênçãos (%d/%d)! Enviando blessing-buy all…", b.config.ID, b.status.CharName, cnt, tot)
				_ = sock.BuyBlessing("all")
			}
		}
	})

	// Morte do jogador: cancela hunt e revive automaticamente na cidade
	sock.On("player-died", func(_ protocol.RawEvent) {
		b.mu.Lock()
		b.status.Deaths++
		b.status.HP = 0
		b.status.Hunt = nil
		b.mu.Unlock()
		log.Printf("[%s: %s] 💀 Personagem morreu! Cancelando hunt e enviando revive…", b.config.ID, b.status.CharName)
		_ = sock.LeaveHunt()
		_ = sock.Send("cancel-hunt", map[string]any{})
		time.Sleep(500 * time.Millisecond)
		_ = sock.Send("revive", map[string]any{})

		// 🛡️ Garante as blessings gratuitas/completas pós-morte:
		go func() {
			time.Sleep(2 * time.Second)
			log.Printf("[%s: %s] 🛡️ Solicitando blessings gratuitas/completas pós-morte (blessing-buy all)…", b.config.ID, b.status.CharName)
			_ = sock.BlessingsOpen()
			time.Sleep(300 * time.Millisecond)
			_ = sock.BuyBlessing("all")
		}()
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

			if hp <= 0 {
				_ = sock.LeaveHunt()
				_ = sock.Send("cancel-hunt", map[string]any{})
				_ = sock.Send("revive", map[string]any{})
			}

			if maxHp > 0 && hp > 0 && float64(hp)/float64(maxHp) < 0.25 && inHunt && !b.retreating {
				b.retreating = true
				log.Printf("[%s] HP < 25%%: Recuando temporariamente da hunt para regeneração de graça (Zero-Waste)!", b.config.ID)
				_ = sock.LeaveHunt()
				go func() {
					time.Sleep(15 * time.Second)
					b.retreating = false
					if !b.config.OnlyTraining {
						targetHunt := HuntOrder[b.currentHuntIndex]
						_ = sock.StartHunt(targetHunt, b.config.Tier)
						log.Printf("[%s] HP regenerado. Retomando caça de bestiário em %s.", b.config.ID, targetHunt)
					}
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

			if data.Kills != nil {
				for k, v := range data.Kills {
					bst.ByMonster[k] = v
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

			tStage := b.config.TargetStage
			if tStage <= 0 {
				tStage = DefaultTargetStage
			}

			baseReq := 2500
			if data.KillsRequired != nil && *data.KillsRequired > 0 {
				baseReq = *data.KillsRequired
			}

			// 0. Redirecionamento Sequencial (Apenas Líder): Encontra a primeira caverna incompleta no estágio alvo
			if b.config.Role != "follower" && !b.config.OnlyTraining {
				firstIncompleteIdx, firstIncompleteHunt := b.getTargetHuntLocked()
				if currentHunt != firstIncompleteHunt {
					log.Printf("[%s] 🔄 Redirecionando Duo para fechar Estágio %d sequencialmente: %s -> %s (Hunt %d/%d)",
						b.config.ID, tStage, currentHunt, firstIncompleteHunt, firstIncompleteIdx+1, len(HuntOrder))
					b.currentHuntIndex = firstIncompleteIdx
					currentHunt = firstIncompleteHunt
					if b.status.Hunt != nil {
						_ = sock.ChangeHunt(firstIncompleteHunt, b.config.Tier)
					} else {
						_ = sock.StartHunt(firstIncompleteHunt, b.config.Tier)
					}
				}
			}

			// Pega o monstro alvo da hunt atual que ainda não atingiu o targetStage
			bestiaryID, monsterName := GetHuntTargetMonster(currentHunt, bst.Stages, bst.ByMonster, tStage, baseReq)
			bst.CurrentMonster = monsterName
			bst.MonsterID = bestiaryID
			if k, ok := bst.ByMonster[bestiaryID]; ok {
				bst.Kills = k
			}
			st := bst.Stages[bestiaryID]
			bst.Stage = st
			bst.TargetStage = tStage
			bst.Required = GetRequiredKillsForStage(st, baseReq)

			huntMonsters := GetHuntMonsters(currentHunt)
			gold := b.status.Gold
			b.mu.Unlock()

			// 1. Auto unlock de estágio do bestiário para TODOS os monstros da hunt até o targetStage
			// Regra Zero-Gasto: Só tenta desbloquear se stage < tStage. Se stage >= 1 (custo de 30k+ gp),
			// só tenta se o char tiver bastante ouro livre (>= 50k gp):
			for _, m := range huntMonsters {
				k := bst.ByMonster[m.BestiaryID]
				stage := bst.Stages[m.BestiaryID]
				req := GetRequiredKillsForStage(stage, baseReq)
				if k >= req && stage < tStage && (stage == 0 || gold >= 50000) {
					now := time.Now()
					if now.Sub(b.lastBestiaryUnlock) > 15*time.Second {
						b.lastBestiaryUnlock = now
						_ = sock.UnlockBestiary(m.BestiaryID)
						bst.Stages[m.BestiaryID] = stage + 1
						_ = sock.RequestCyclopedia()
						rewardType := "+1% XP Permanente"
						if (stage+1)%4 == 0 {
							rewardType = "⚔️ +1% DANO TOTAL Permanente"
						}
						log.Printf("[%s] ⭐ [BESTIÁRIO] Desbloqueio enviado para %s [Estágio %d/%d] (%d/%d kills) [%s]", b.config.ID, m.Name, stage+1, tStage, k, req, rewardType)
					}
				}
			}

			// 2. Só avança para a próxima hunt quando TODOS os monstros daquela caverna atingirem o targetStage
			if b.config.Role != "follower" && !b.config.OnlyTraining {
				allCompleted := true
				targetKills := GetRequiredKillsForStage(tStage-1, baseReq)
				for _, m := range huntMonsters {
					stage := bst.Stages[m.BestiaryID]
					k := bst.ByMonster[m.BestiaryID]
					if stage < tStage && k < targetKills {
						allCompleted = false
						break
					}
				}

				if allCompleted && len(huntMonsters) > 0 {
					log.Printf("[%s] 🏆 [BESTIÁRIO] TODOS os monstros de %s concluídos no Estágio %d! Avançando para a próxima hunt…", b.config.ID, currentHunt, tStage)
					b.advanceToNextHunt(sock)
				}
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
				log.Printf("[%s: %s] ⭐ [XP GANHO] +%d XP!", b.config.ID, b.status.CharName, gain)
				b.mu.Lock()
				b.status.Analyzer.Experience += gain
				b.status.Analyzer.Kills++
				b.status.Experience += gain
				b.status.Bestiary.Kills++
				b.status.Bestiary.TotalKills++
				if b.status.Bestiary.MonsterID != "" {
					b.status.Bestiary.ByMonster[b.status.Bestiary.MonsterID]++
				}
				b.sessionExpired = false
				b.mu.Unlock()
			}
		}
	})
}

func (b *Bot) advanceToNextHunt(sock *client.SocketClient) {
	if b.config.Role == "follower" || b.config.OnlyTraining {
		return
	}
	tStage := b.config.TargetStage
	if tStage <= 0 {
		tStage = DefaultTargetStage
	}
	if b.currentHuntIndex+1 < len(HuntOrder) {
		b.currentHuntIndex++
		nextHunt := HuntOrder[b.currentHuntIndex]
		b.mu.Lock()
		req := 2500
		if b.status.Bestiary != nil && b.status.Bestiary.Required > 0 {
			req = b.status.Bestiary.Required
		}
		bestiaryID, monsterName := GetHuntTargetMonster(nextHunt, b.status.Bestiary.Stages, b.status.Bestiary.ByMonster, tStage, req)
		b.status.Bestiary.MonsterID = bestiaryID
		b.status.Bestiary.CurrentMonster = monsterName
		b.status.Bestiary.TargetStage = tStage
		if k, ok := b.status.Bestiary.ByMonster[bestiaryID]; ok {
			b.status.Bestiary.Kills = k
		}
		inHunt := b.status.Hunt != nil
		b.mu.Unlock()
		log.Printf("[%s] 🚀 [BESTIÁRIO] Avançando para: %s (%d/%d) [Alvo: %s (Estágio %d)]", b.config.ID, nextHunt, b.currentHuntIndex+1, len(HuntOrder), monsterName, tStage)
		if inHunt {
			_ = sock.ChangeHunt(nextHunt, b.config.Tier)
		} else {
			_ = sock.StartHunt(nextHunt, b.config.Tier)
		}
	} else {
		log.Printf("[%s] 🎉 TODOS OS BESTIÁRIOS CONCLUÍDOS NO ESTÁGIO %d COM SUCESSO!", b.config.ID, tStage)
	}
}
