package bot

var HuntOrder = []string{
	// 💎 Top Lucro, Zero-Gasto & Zero-Morte (Bestiário Sequencial Seguro)
	"amazon-hunt",      // 1. Protective Charm (6.1k gp) - Lv 20+ (0 mortes, farm massivo)
	"swamp-troll-hunt", // 2. Medicine Pouch (4k gp) - Lv 10+ (0 mortes)
	"tarantula-hunt",   // 3. Tarantula Eggs & Fangs - Lv 25+ (0 mortes)
	"cult-hunt",        // 4. Rope Belt (7.7k - 15k gp) - Apenas Lv 75+ (quando o time aguentar)
	"mutated-cave-hunt",// 5. Sabretooth (8.9k - 20k gp) - Apenas Lv 80+
	"vampire-hunt",     // 6. Vampire Teeth & BoH - Apenas Lv 80+
	// Sequência normal de hunts
	"spider-hunt", "troll-hunt", "orc-hunt", "folda-hunt",
	"skeleton-hunt", "rotworm-hunt", "dwarf-hunt", "minotaur-hunt", "gloom-ghost-wolf-hunt",
	"dark-cathedral-hunt", "ghoul-hunt", "rorc-hunt", "yalahar-elf-hunt",
	"tarantula-hunt", "scarab-hunt", "swampling-hunt", "tortoise-hunt", "mutated-human-hunt",
	"cyclops-hunt", "mummy-hunt", "bonelord-hunt", "orc-fortress-hunt", "green-djinn-hunt",
	"blue-djinn-hunt", "carlin-corym-hunt", "elder-forest-hunt", "drefia-hunt",
	"ab-bonelord-hunt", "ice-golem-hunt", "lizard-steppe-hunt", "brimstone-cave-hunt",
	"dragon-hunt", "bog-raider-hunt", "giant-spider-hunt",
	"deeplings-hunt", "hero-hunt", "wyrm-hunt", "zao-stronghold-hunt", "grimvale-warrens-hunt",
	"rathleton-minotaurs-hunt", "grimvale-dens-hunt", "dragon-lord-hunt", "war-golem-hunt",
	"lizard-chosen-hunt", "werehyaena-hunt", "werelion-hunt", "behemoth-hunt", "hellspawn-hunt",
	"hydra-hunt", "seacrest-serpent-hunt", "draken-walls-hunt", "ripper-spectre-hunt",
	"goroma-serpent-spawn-hunt", "asura-hunt", "gazer-spectre-hunt", "roshamuul-lower-hunt",
	"burster-spectre-hunt", "grim-reaper-hunt", "demon-hunt", "falcon-hunt",
	"falcon-bastion-hunt", "cobra-bastion-hunt", "hell-hub-hunt", "catacombs-hunt",
	"issavi-hunt", "issavi-south-hunt", "ice-library-hunt",
	"rat-hunt", // Rato em último
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
	Stage             int            `json:"stage"`
	TargetStage       int            `json:"targetStage"`
	Stages            map[string]int `json:"stages"`
	ByMonster         map[string]int `json:"byMonster"`
}

type AnalyzerState struct {
	Kills      int `json:"kills"`
	Experience int `json:"experience"`
	LootValue  int `json:"lootValue"`
	Waste      int `json:"waste"`
}

type InventorySlot struct {
	UID         any    `json:"uid"`
	ItemID      int    `json:"itemId"`
	Name        string `json:"name,omitempty"`
	Slot        string `json:"slot,omitempty"`
	Attack      int    `json:"attack,omitempty"`
	Defense     int    `json:"defense,omitempty"`
	AttackSkill string `json:"attackSkill,omitempty"`
	Count       int    `json:"count"`
	SellLocked  bool   `json:"sellLocked"`
	Bound       bool   `json:"bound"`
	Tier        int    `json:"tier"`
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
	OnlyTraining       bool           `json:"onlyTraining,omitempty"`
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
	TargetStage  int      `json:"targetStage,omitempty"`
	PartyMembers []string `json:"-"`
	Role         string   `json:"role,omitempty"`        // "leader" ou "follower"
	PartnerName  string   `json:"partnerName,omitempty"` // Nome do char parceiro da Duo
	OnlyTraining bool     `json:"onlyTraining,omitempty"`// Se true, treina skills 100% do tempo (não caça)
}
