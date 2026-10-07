package bot

var HuntOrder = []string{
	"rat-hunt", "spider-hunt", "troll-hunt", "swamp-troll-hunt", "orc-hunt", "folda-hunt",
	"skeleton-hunt", "rotworm-hunt", "dwarf-hunt", "minotaur-hunt", "gloom-ghost-wolf-hunt",
	"amazon-hunt", "dark-cathedral-hunt", "ghoul-hunt", "rorc-hunt", "yalahar-elf-hunt",
	"tarantula-hunt", "scarab-hunt", "swampling-hunt", "tortoise-hunt", "mutated-human-hunt",
	"cyclops-hunt", "mummy-hunt", "bonelord-hunt", "orc-fortress-hunt", "green-djinn-hunt",
	"blue-djinn-hunt", "carlin-corym-hunt", "cult-hunt", "elder-forest-hunt", "drefia-hunt",
	"ab-bonelord-hunt", "ice-golem-hunt", "lizard-steppe-hunt", "brimstone-cave-hunt",
	"dragon-hunt", "vampire-hunt", "mutated-cave-hunt", "bog-raider-hunt", "giant-spider-hunt",
	"deeplings-hunt", "hero-hunt", "wyrm-hunt", "zao-stronghold-hunt", "grimvale-warrens-hunt",
	"rathleton-minotaurs-hunt", "grimvale-dens-hunt", "dragon-lord-hunt", "war-golem-hunt",
	"lizard-chosen-hunt", "werehyaena-hunt", "werelion-hunt", "behemoth-hunt", "hellspawn-hunt",
	"hydra-hunt", "seacrest-serpent-hunt", "draken-walls-hunt", "ripper-spectre-hunt",
	"goroma-serpent-spawn-hunt", "asura-hunt", "gazer-spectre-hunt", "roshamuul-lower-hunt",
	"burster-spectre-hunt", "grim-reaper-hunt", "demon-hunt", "falcon-hunt",
	"falcon-bastion-hunt", "cobra-bastion-hunt", "hell-hub-hunt", "catacombs-hunt",
	"issavi-hunt", "issavi-south-hunt", "ice-library-hunt",
}

type HuntState struct {
	HuntID     string `json:"huntId"`
	Tier       int    `json:"tier"`
	InstanceID string `json:"instanceId,omitempty"`
}

type BestiaryState struct {
	CurrentMonster    string         `json:"currentMonster"`
	MonsterID         string         `json:"monsterId"`
	Kills             int            `json:"kills"`
	Required          int            `json:"required"`
	BonusPercent      int            `json:"bonusPercent"`
	CompletedMonsters int            `json:"completedMonsters"`
	TotalMonsters     int            `json:"totalMonsters"`
	TotalKills        int            `json:"totalKills"`
	Stages            map[string]int `json:"stages"`
	ByMonster         map[string]int `json:"byMonster"`
}

type AnalyzerState struct {
	Kills      int `json:"kills"`
	Experience int `json:"experience"`
	LootValue  int `json:"lootValue"`
	Waste      int `json:"waste"`
}

type AccountStatus struct {
	ID                 string         `json:"id"`
	CharName           string         `json:"charName"`
	Level              int            `json:"level"`
	Experience         int            `json:"experience"`
	ExperienceNeeded   int            `json:"experienceNeeded"`
	InGame             bool           `json:"inGame"`
	HP                 int            `json:"hp"`
	MaxHP              int            `json:"maxHp"`
	Mana               int            `json:"mana"`
	MaxMana            int            `json:"maxMana"`
	Gold               int            `json:"gold"`
	Hunt               *HuntState     `json:"hunt,omitempty"`
	StaminaMs          int64          `json:"staminaMs"`
	SessionRemainingMs int64          `json:"sessionRemainingMs"`
	StaminaRefillCost  int            `json:"staminaRefillCost"`
	StaminaRefillsLeft int            `json:"staminaRefillsLeft"`
	Bestiary           *BestiaryState `json:"bestiary"`
	Analyzer           *AnalyzerState `json:"analyzer"`
	LootsTaken         int            `json:"lootsTaken"`
	Deaths             int            `json:"deaths"`
	UpdatedAt          int64          `json:"updatedAt"`
}

type AccountConfig struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	CharName    string `json:"charName,omitempty"`
	HuntID      string `json:"huntId,omitempty"`
	Tier        int    `json:"tier,omitempty"`
	AutoStamina bool   `json:"autoStamina,omitempty"`
}
