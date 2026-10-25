package bot

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// TeamCoordinator synchronizes stamina and hunt/training state across all team accounts.
// Regra: "Assim que a stamina acabar de um, todos devem ir treinar. Quando recuperar, todos voltam a caçar."
type TeamCoordinator struct {
	mu         sync.RWMutex
	bots       []*Bot
	resting    bool
	restReason string
	lastLog    time.Time
}

var GlobalCoordinator = &TeamCoordinator{}

func (c *TeamCoordinator) RegisterBot(b *Bot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bots = append(c.bots, b)
}

// ShouldTeamRest checks if ANY active member in the team has stamina < 300,000 ms (5 min, game limit).
func (c *TeamCoordinator) ShouldTeamRest() (bool, string, int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, b := range c.bots {
		st := b.GetStatus()
		if !st.InGame {
			continue
		}
		// Se qualquer conta tiver stamina baixa (< 5 min / 300.000 ms)
		if st.StaminaMs > 0 && st.StaminaMs < 300000 {
			return true, fmt.Sprintf("%s (%s)", b.config.ID, st.CharName), st.StaminaMs
		}
	}
	return false, "", 0
}

// CanTeamResume checks if ALL active members have recovered stamina above the threshold.
// minMs: e.g. 600,000 ms (10 min) to avoid immediate thrashing after entering.
func (c *TeamCoordinator) CanTeamResume(minMs int64) (bool, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.bots) == 0 {
		return true, ""
	}

	for _, b := range c.bots {
		st := b.GetStatus()
		if !st.InGame {
			continue
		}
		if st.StaminaMs < minMs {
			return false, fmt.Sprintf("%s (%s) com apenas %d ms", b.config.ID, st.CharName, st.StaminaMs)
		}
	}
	return true, ""
}

func (c *TeamCoordinator) IsTeamResting() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.resting
}

func (c *TeamCoordinator) SetTeamResting(resting bool, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resting = resting
	c.restReason = reason
}

// GetTeamMinLevel returns the lowest level among all active team members
func (c *TeamCoordinator) GetTeamMinLevel() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	minLvl := 9999
	for _, b := range c.bots {
		st := b.GetStatus()
		if st.InGame && st.Level > 0 {
			if st.Level < minLvl {
				minLvl = st.Level
			}
		}
	}
	if minLvl == 9999 {
		return 1
	}
	return minLvl
}

// GetTeamTotalGold returns the sum of gold across all team members
func (c *TeamCoordinator) GetTeamTotalGold() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := 0
	for _, b := range c.bots {
		st := b.GetStatus()
		total += st.Gold
	}
	return total
}


// ForceTeamRest forces all accounts in the team to leave hunt and enter idle training.
func (c *TeamCoordinator) ForceTeamRest(reason string) {
	c.mu.Lock()
	c.resting = true
	c.restReason = reason
	bots := make([]*Bot, len(c.bots))
	copy(bots, c.bots)
	c.mu.Unlock()

	log.Printf("🚨 [COORDENADOR 4-VOC] Ordem de Treino Coletivo: %s. Todos os 4 personagens entrando em Treino Idle!", reason)
	for _, b := range bots {
		b.mu.Lock()
		b.sessionExpired = true
		b.mu.Unlock()
		if b.socket != nil {
			_ = b.socket.LeaveHunt()
			_ = b.socket.SetIdleTraining(true)
		}
	}
}
