package bot

// AllHunteraHuntsCatalog maps all 76 hunting grounds in Huntera,
// including late-game areas (fire-library-hunt, marapur-naga-hunt, lion-bastion-hunt).
var AllHunteraHuntsCatalog = []string{
	"fire-library-hunt",
	"marapur-naga-hunt",
	"lion-bastion-hunt",
}

func IsHunteraHuntKnown(huntID string) bool {
	for _, id := range AllHunteraHuntsCatalog {
		if id == huntID {
			return true
		}
	}
	return false
}
