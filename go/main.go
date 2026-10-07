package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"huntera-bot/pkg/bot"
	"huntera-bot/pkg/server"
)

//go:embed accounts.json
var defaultAccountsJSON []byte

type BotManager struct {
	bots []*bot.Bot
}

func (m *BotManager) GetAllStatuses() []bot.AccountStatus {
	var list []bot.AccountStatus
	for _, b := range m.bots {
		list = append(list, b.GetStatus())
	}
	return list
}

func formatGp(n int) string {
	a := n
	if a < 0 {
		a = -a
	}
	if a < 1000 {
		return fmt.Sprintf("%d gp", n)
	}
	if a < 1_000_000 {
		return fmt.Sprintf("%.1fk gp", float64(n)/1000.0)
	}
	return fmt.Sprintf("%.2fkk gp", float64(n)/1_000_000.0)
}

func formatStamina(ms int64) string {
	if ms <= 0 {
		return "—"
	}
	totalMins := ms / 60000
	h := totalMins / 60
	m := totalMins % 60
	return fmt.Sprintf("%dh %02dm", h, m)
}

func main() {
	log.Println("================================================================================")
	log.Println("     ⚔  HUNTERA MULTI-BOT NATIVO EM GO (HIGH-PERFORMANCE / ULTRA-LEVE) ⚔        ")
	log.Println("================================================================================")

	// 1. Carregar contas
	var accountConfigs []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Email         string `json:"email"`
		Password      string `json:"password"`
		CharacterName string `json:"characterName"`
		HuntID        string `json:"huntId"`
		Tier          int    `json:"tier"`
		AutoStamina   bool   `json:"autoStamina"`
	}

	content := defaultAccountsJSON
	if diskData, err := os.ReadFile("accounts.json"); err == nil {
		content = diskData
	}

	if err := json.Unmarshal(content, &accountConfigs); err != nil {
		log.Fatalf("Falha ao carregar accounts.json: %v", err)
	}

	targetAcc := os.Getenv("ACCOUNT")
	var activeConfigs []bot.AccountConfig

	for _, a := range accountConfigs {
		if targetAcc != "" && a.ID != targetAcc {
			continue
		}
		charName := a.CharacterName
		if charName == "" {
			charName = a.Name
		}
		activeConfigs = append(activeConfigs, bot.AccountConfig{
			ID:          a.ID,
			Email:       a.Email,
			Password:    a.Password,
			CharName:    charName,
			HuntID:      a.HuntID,
			Tier:        a.Tier,
			AutoStamina: true,
		})
	}

	if len(activeConfigs) == 0 {
		log.Fatalf("Nenhuma conta configurada para rodar.")
	}

	log.Printf("Iniciando %d conta(s) simultânea(s) em Goroutines nativas…\n", len(activeConfigs))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := &BotManager{}
	for _, cfg := range activeConfigs {
		b := bot.NewBot(cfg)
		manager.bots = append(manager.bots, b)
		go b.Run(ctx)
	}

	// 2. Servidor Web HTTP Nativo (Porta 3000)
	srv := server.NewServer(3000, manager)
	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("[Web Server] Erro: %v", err)
		}
	}()

	// 3. Monitor Periódico no Terminal com estatísticas de RAM
	go func() {
		time.Sleep(3 * time.Second)
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				allocMB := float64(m.Alloc) / 1024.0 / 1024.0
				sysMB := float64(m.Sys) / 1024.0 / 1024.0

				statuses := manager.GetAllStatuses()
				totalGold := 0
				totalKills := 0
				inGameCount := 0

				fmt.Printf("\n\033[1;36m================================================================================\033[0m\n")
				fmt.Printf("\033[1;36m       ⚔  HUNTERA GO ENGINE — RAM: %.1f MB (Sys: %.1f MB) — Goroutines: %d ⚔   \033[0m\n",
					allocMB, sysMB, runtime.NumGoroutine())
				fmt.Printf("\033[1;36m================================================================================\033[0m\n")

				for _, st := range statuses {
					totalGold += st.Gold
					b := st.Bestiary
					charKills := 0
					if b != nil {
						charKills = b.TotalKills
						totalKills += charKills
					}
					if st.InGame {
						inGameCount++
					}

					huntStr := "Aguardando"
					if st.Hunt != nil {
						huntStr = fmt.Sprintf("%s (tier %d)", st.Hunt.HuntID, st.Hunt.Tier)
					}

					bonus := 0
					comp := 0
					totalMonst := 164
					currMonst := "—"
					bKills := 0
					bReq := 2500
					if b != nil {
						bonus = b.BonusPercent
						comp = b.CompletedMonsters
						totalMonst = b.TotalMonsters
						currMonst = b.CurrentMonster
						bKills = b.Kills
						bReq = b.Required
					}

					statusBadge := "\033[1;33m[ CONECTANDO ]\033[0m"
					if st.InGame {
						statusBadge = "\033[1;32m[ EM JOGO ]\033[0m"
					}

					fmt.Printf("[%s: \033[1m%s\033[0m (Lv. %d)] %s\n", st.ID, st.CharName, st.Level, statusBadge)
					fmt.Printf("  💰 Ouro: \033[1;33m%s\033[0m (%d gp) │ XP: %d/%d │ Stamina: %s\n",
						formatGp(st.Gold), st.Gold, st.Experience, st.ExperienceNeeded, formatStamina(st.StaminaMs))
					fmt.Printf("  ⭐ Bestiário: \033[1;35m%s\033[0m [%d/%d] │ Bônus: \033[1;32m+%d%% XP\033[0m │ 100%%: %d/%d │ Kills: %d\n",
						currMonst, bKills, bReq, bonus, comp, totalMonst, charKills)
					fmt.Printf("  🗡️ Hunt: \033[36m%s\033[0m │ Zero-Waste: 100%%\n", huntStr)
					fmt.Println("--------------------------------------------------------------------------------")
				}

				fmt.Printf("💰 TOTAL OURO (4 CONTAS): \033[1;33m%s\033[0m (%d gp) │ 💀 KILLS TOTAIS: %d │ ATIVAS: %d/%d\n",
					formatGp(totalGold), totalGold, totalKills, inGameCount, len(statuses))
				fmt.Printf("\033[1;36m================================================================================\033[0m\n\n")
			}
		}
	}()

	// 4. Captura de encerramento seguro (SIGINT / SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("\nEncerrando Huntera Go Bot graciosamente…")
	cancel()
	time.Sleep(500 * time.Millisecond)
	log.Println("✓ Bot finalizado com segurança.")
}
