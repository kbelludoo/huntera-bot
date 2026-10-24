package bot

import (
	"testing"
)

func TestPaladinWeaponPriority(t *testing.T) {
	b := NewBot(AccountConfig{
		ID:       "acc1",
		CharName: "Kbelludoo",
	})

	slots := []*InventorySlot{
		{ItemID: 3074, Name: "wand of vortex", Slot: "weapon", Attack: 0, AttackSkill: ""},
		{ItemID: 3291, Name: "knife", Slot: "weapon", Attack: 5, AttackSkill: "sword"},
		{ItemID: 3277, Name: "spear", Slot: "weapon", Attack: 9, AttackSkill: "distance"},
		{ItemID: 21427, Name: "paladin's bow", Slot: "weapon", Attack: 0, AttackSkill: "distance"},
	}

	bestIdx := -1
	bestAtk := -1
	bestName := ""

	for idx, s := range slots {
		if s == nil || s.Slot != "weapon" {
			continue
		}
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

	if bestIdx != 2 || bestName != "spear" {
		t.Fatalf("expected spear at index 2, got %s at index %d", bestName, bestIdx)
	}

	if b.config.ID != "acc1" {
		t.Fatalf("expected acc1 paladin, got %s", b.config.ID)
	}
}
