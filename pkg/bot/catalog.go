package bot

import (
	_ "embed"
	"encoding/json"
)

//go:embed catalog.json
var catalogJSON []byte

type CatalogMonster struct {
	Name       string `json:"name"`
	BestiaryID string `json:"bestiaryId"`
}

type CatalogHunt struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Monsters []CatalogMonster `json:"monsters"`
}

type HuntCatalog struct {
	Hunts []CatalogHunt `json:"hunts"`
}

var (
	huntMonsterMap     = make(map[string]CatalogMonster)
	huntAllMonstersMap = make(map[string][]CatalogMonster)
)

func init() {
	var cat HuntCatalog
	if err := json.Unmarshal(catalogJSON, &cat); err == nil {
		for _, h := range cat.Hunts {
			if len(h.Monsters) > 0 {
				huntMonsterMap[h.ID] = h.Monsters[0]
				huntAllMonstersMap[h.ID] = h.Monsters
			}
		}
	}
}

// GetHuntMonsters returns all monsters present in a hunt cave.
func GetHuntMonsters(huntID string) []CatalogMonster {
	if list, ok := huntAllMonstersMap[huntID]; ok && len(list) > 0 {
		return list
	}
	return []CatalogMonster{{Name: "Rat", BestiaryID: "rat"}}
}

const DefaultTargetStage = 1

// GetRequiredKillsForStage returns the cumulative kill target for each bestiary stage using the game formula:
// kills = baseKills * 2^stage
// Estágio 1: 2.500 | Estágio 2: 5.000 | Estágio 3: 10.000 | Estágio 4: 20.000 (+Dmg)
// Estágio 5: 40.000 | Estágio 6: 80.000 | Estágio 7: 160.000
func GetRequiredKillsForStage(stage int, baseKills int) int {
	if baseKills <= 0 {
		baseKills = 2500
	}
	if stage < 0 {
		stage = 0
	}
	if stage > 15 {
		stage = 15
	}
	return baseKills * (1 << stage)
}

// GetHuntTargetMonster returns the current target monster in the hunt that still needs bestiary kills to reach targetStage.
func GetHuntTargetMonster(huntID string, stages map[string]int, kills map[string]int, targetStage int, baseKills int) (bestiaryID, name string) {
	if targetStage <= 0 {
		targetStage = DefaultTargetStage
	}
	targetKills := GetRequiredKillsForStage(targetStage-1, baseKills)
	monsters := GetHuntMonsters(huntID)
	for _, m := range monsters {
		st := stages[m.BestiaryID]
		k := kills[m.BestiaryID]
		if st < targetStage && k < targetKills {
			return m.BestiaryID, m.Name
		}
	}
	if len(monsters) > 0 {
		return monsters[0].BestiaryID, monsters[0].Name
	}
	return "rat", "Rat"
}

// GetHuntMonster returns the first monster's bestiaryId and Name for a given huntId.
func GetHuntMonster(huntID string) (bestiaryID, name string) {
	if m, ok := huntMonsterMap[huntID]; ok {
		return m.BestiaryID, m.Name
	}
	return "rat", "Rat"
}

var huntMinLevels = map[string]int{
	"amazon-hunt":       20,
	"swamp-troll-hunt":  10,
	"tarantula-hunt":    25,
	"cult-hunt":         65, // Cultists dão dano alto de morte/lifedrain; seguro a partir do lv 65 com skills altas
	"mutated-cave-hunt": 80, // Mutated Bat/Tiger seguro a partir do lv 80
	"vampire-hunt":      80, // Vampires seguro a partir do lv 80
}

// FindFirstIncompleteHunt scans HuntOrder from index 0 and returns the first hunt where at least one monster has not reached targetStage.
func FindFirstIncompleteHunt(stages map[string]int, kills map[string]int, targetStage int, baseKills int, charLevel int) (int, string) {
	if targetStage <= 0 {
		targetStage = DefaultTargetStage
	}
	targetKills := GetRequiredKillsForStage(targetStage-1, baseKills)

	for i, huntID := range HuntOrder {
		if reqLvl, ok := huntMinLevels[huntID]; ok && charLevel > 0 && charLevel < reqLvl {
			continue // Char ainda não tem level suficiente para esta hunt
		}
		monsters := GetHuntMonsters(huntID)
		allCompleted := true
		for _, m := range monsters {
			st := stages[m.BestiaryID]
			k := kills[m.BestiaryID]
			if st < targetStage && k < targetKills {
				allCompleted = false
				break
			}
		}
		if !allCompleted {
			return i, huntID
		}
	}
	return len(HuntOrder) - 1, HuntOrder[len(HuntOrder)-1]
}
