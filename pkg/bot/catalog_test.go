package bot

import (
	"testing"
)

func TestGetHuntMonster(t *testing.T) {
	tests := []struct {
		huntID     string
		wantID     string
		wantName   string
	}{
		{"rat-hunt", "rat", "Rat"},
		{"spider-hunt", "spider", "Spider"},
		{"troll-hunt", "troll", "Troll"},
		{"skeleton-hunt", "skeleton", "Skeleton"},
		{"swamp-troll-hunt", "swampTroll", "Swamp Troll"},
	}

	for _, tt := range tests {
		gotID, gotName := GetHuntMonster(tt.huntID)
		if gotID != tt.wantID || gotName != tt.wantName {
			t.Errorf("GetHuntMonster(%q) = (%q, %q), want (%q, %q)", tt.huntID, gotID, gotName, tt.wantID, tt.wantName)
		}
	}
}
