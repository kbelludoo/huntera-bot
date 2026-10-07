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
	huntMonsterMap = make(map[string]CatalogMonster)
)

func init() {
	var cat HuntCatalog
	if err := json.Unmarshal(catalogJSON, &cat); err == nil {
		for _, h := range cat.Hunts {
			if len(h.Monsters) > 0 {
				huntMonsterMap[h.ID] = h.Monsters[0]
			}
		}
	}
}

// GetHuntMonster returns the first monster's bestiaryId and Name for a given huntId.
func GetHuntMonster(huntID string) (bestiaryID, name string) {
	if m, ok := huntMonsterMap[huntID]; ok {
		return m.BestiaryID, m.Name
	}
	return "rat", "Rat"
}
