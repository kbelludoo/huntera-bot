package bot

import (
	"testing"
)

func TestGetBestProfitHunt(t *testing.T) {
	tests := []struct {
		level        int
		expectedHunt string
	}{
		{level: 160, expectedHunt: "asura-hunt"},
		{level: 125, expectedHunt: "hero-hunt"},
		{level: 95, expectedHunt: "mutated-cave-hunt"},
		{level: 70, expectedHunt: "cult-hunt"},
		{level: 30, expectedHunt: "tarantula-hunt"},
		{level: 20, expectedHunt: "amazon-hunt"},
		{level: 10, expectedHunt: "swamp-troll-hunt"},
	}

	for _, tt := range tests {
		huntID, name := GetBestProfitHunt(tt.level)
		if huntID != tt.expectedHunt {
			t.Errorf("GetBestProfitHunt(%d) = %s (%s); want %s", tt.level, huntID, name, tt.expectedHunt)
		}
	}
}

func TestIsVipFarmModeActive(t *testing.T) {
	b := &Bot{
		config: AccountConfig{ID: "acc3", Role: "leader"},
		status: AccountStatus{VipDays: 0},
	}

	// Fase 1: Sem VIP -> VIP Farm Mode deve estar ATIVO
	if !b.IsVipFarmModeActive() {
		t.Errorf("Expected IsVipFarmModeActive() = true when VipDays = 0")
	}

	// Fase 2: Com 30 dias de VIP -> VIP Farm Mode deve continuar ATIVO até consolidar 90 dias
	b.status.VipDays = 30
	if !b.IsVipFarmModeActive() {
		t.Errorf("Expected IsVipFarmModeActive() = true when VipDays = 30")
	}

	// Fase 3: Com 90 dias de VIP consolidados -> Libera rotação completa de Bestiário
	b.status.VipDays = 90
	if b.IsVipFarmModeActive() {
		t.Errorf("Expected IsVipFarmModeActive() = false when VipDays = 90")
	}

	// Se explicitamente desligado na config
	disabled := false
	b.config.FarmVIPMode = &disabled
	b.status.VipDays = 0
	if b.IsVipFarmModeActive() {
		t.Errorf("Expected IsVipFarmModeActive() = false when config.FarmVIPMode = false")
	}
}
