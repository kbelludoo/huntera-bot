package protocol

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed opcodes.json
var opcodesJSON []byte

type OpcodesData struct {
	Incoming map[string]int `json:"incoming"`
	Outgoing map[string]int `json:"outgoing"`
}

var (
	IncomingByCode = make(map[int]string)
	OutgoingByName = make(map[string]int)
)

const ClientVersion = "0.3.0+e0"

func init() {
	var data OpcodesData
	if err := json.Unmarshal(opcodesJSON, &data); err != nil {
		panic(fmt.Sprintf("failed to parse opcodes.json: %v", err))
	}

	for name, code := range data.Incoming {
		IncomingByCode[code] = name
	}
	for name, code := range data.Outgoing {
		OutgoingByName[name] = code
	}
}
