package bot

import (
	"testing"
)

func TestGetVipTargetPriority(t *testing.T) {
	coord := &TeamCoordinator{}

	b1 := &Bot{config: AccountConfig{ID: "acc1"}, status: AccountStatus{CharName: "Kbelludoo", VipDays: 0}}
	b2 := &Bot{config: AccountConfig{ID: "acc2"}, status: AccountStatus{CharName: "Kbelludoos", VipDays: 0}}
	b3 := &Bot{config: AccountConfig{ID: "acc3"}, status: AccountStatus{CharName: "Kbelludook", VipDays: 0}}
	b4 := &Bot{config: AccountConfig{ID: "acc4"}, status: AccountStatus{CharName: "Kbelludoom", VipDays: 0}}

	coord.RegisterBot(b1)
	coord.RegisterBot(b2)
	coord.RegisterBot(b3)
	coord.RegisterBot(b4)

	// 1. Inicial: Ninguém tem VIP -> Alvo deve ser o Líder (acc3 - Knight)
	targetChar, targetID := coord.GetVipTarget()
	if targetID != "acc3" || targetChar != "Kbelludook" {
		t.Fatalf("Expected VIP target acc3 (Kbelludook), got %s (%s)", targetID, targetChar)
	}

	// 2. Líder comprou 30 dias de VIP -> Próximo alvo deve ser o Paladino (acc1)
	b3.status.VipDays = 30
	b3.status.VipActive = true
	targetChar, targetID = coord.GetVipTarget()
	if targetID != "acc1" || targetChar != "Kbelludoo" {
		t.Fatalf("Expected VIP target acc1 (Kbelludoo), got %s (%s)", targetID, targetChar)
	}

	// 3. Paladino comprou 30 dias de VIP -> Próximo alvo deve ser o Sorcerer (acc4)
	b1.status.VipDays = 30
	b1.status.VipActive = true
	targetChar, targetID = coord.GetVipTarget()
	if targetID != "acc4" || targetChar != "Kbelludoom" {
		t.Fatalf("Expected VIP target acc4 (Kbelludoom), got %s (%s)", targetID, targetChar)
	}

	// 4. Sorcerer comprou 30 dias de VIP -> Próximo alvo deve ser o Druid (acc2)
	b4.status.VipDays = 30
	b4.status.VipActive = true
	targetChar, targetID = coord.GetVipTarget()
	if targetID != "acc2" || targetChar != "Kbelludoos" {
		t.Fatalf("Expected VIP target acc2 (Kbelludoos), got %s (%s)", targetID, targetChar)
	}

	// 5. Todos têm 30 dias de VIP -> Fase 2 começa (90 dias): Alvo volta para o Líder (acc3)
	b2.status.VipDays = 30
	b2.status.VipActive = true
	targetChar, targetID = coord.GetVipTarget()
	if targetID != "acc3" || targetChar != "Kbelludook" {
		t.Fatalf("Expected Phase 2 VIP target acc3 (Kbelludook), got %s (%s)", targetID, targetChar)
	}
}

func TestIsTeamMember(t *testing.T) {
	coord := &TeamCoordinator{}
	b1 := &Bot{config: AccountConfig{ID: "acc1"}, status: AccountStatus{CharName: "Kbelludoo"}}
	b3 := &Bot{config: AccountConfig{ID: "acc3"}, status: AccountStatus{CharName: "Kbelludook"}}

	coord.RegisterBot(b1)
	coord.RegisterBot(b3)

	if !coord.IsTeamMember("kbelludoo") {
		t.Errorf("Expected IsTeamMember('kbelludoo') = true")
	}
	if !coord.IsTeamMember("Kbelludook") {
		t.Errorf("Expected IsTeamMember('Kbelludook') = true")
	}
	if coord.IsTeamMember("RandomPlayer") {
		t.Errorf("Expected IsTeamMember('RandomPlayer') = false")
	}
}
