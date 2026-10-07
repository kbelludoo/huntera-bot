package protocol

import (
	"encoding/json"
	"testing"
)

func TestEncodeAndDecodePacket(t *testing.T) {
	// 1. Test ping encode
	type PingPayload struct {
		Timestamp int64 `json:"t"`
	}
	encoded, err := EncodePacket("ping", PingPayload{Timestamp: 123456789})
	if err != nil {
		t.Fatalf("failed to encode ping: %v", err)
	}
	if len(encoded) < 5 {
		t.Fatalf("encoded packet too small: %d bytes", len(encoded))
	}

	// 2. Test mock server message decode (e.g. player-stats code 77)
	code := OutgoingByName["ping"]
	if code == 0 {
		t.Fatalf("ping code is 0")
	}

	mockPayload := map[string]any{
		"level":            92,
		"health":           590,
		"experience":       185153,
		"experienceNeeded": 409600,
	}
	rawMsg, _ := json.Marshal([]any{77, mockPayload})

	// Manually frame it as server would
	seed := uint32(987654321)
	frame := make([]byte, 5+len(rawMsg))
	frame[0] = byte(seed)
	frame[1] = byte(seed >> 8)
	frame[2] = byte(seed >> 16)
	frame[3] = byte(seed >> 24)
	frame[4] = 0 // no compress, no batch
	copy(frame[5:], rawMsg)

	XORCipher(frame[4:], seed)

	events, err := DecodeFrame(frame)
	if err != nil {
		t.Fatalf("failed to decode frame: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "player-stats" {
		t.Fatalf("expected player-stats, got %s", events[0].Type)
	}
	if events[0].Opcode != 77 {
		t.Fatalf("expected opcode 77, got %d", events[0].Opcode)
	}

	var stats struct {
		Level            int `json:"level"`
		Experience       int `json:"experience"`
		ExperienceNeeded int `json:"experienceNeeded"`
	}
	if err := events[0].Unmarshal(&stats); err != nil {
		t.Fatalf("failed to unmarshal stats: %v", err)
	}
	if stats.Level != 92 || stats.Experience != 185153 || stats.ExperienceNeeded != 409600 {
		t.Fatalf("unmarshaled values incorrect: %+v", stats)
	}
}
