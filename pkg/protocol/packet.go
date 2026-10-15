package protocol

import (
	"bytes"
	"compress/flate"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// RawEvent represents a decoded incoming event from the game server.
type RawEvent struct {
	Type    string          `json:"type"`
	Opcode  int             `json:"opcode"`
	Payload json.RawMessage `json:"payload"`
}

// Unmarshal parses the event payload into target struct.
func (e *RawEvent) Unmarshal(v any) error {
	if len(e.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(e.Payload, v)
}

// AsMap parses the event payload into a generic map.
func (e *RawEvent) AsMap() (map[string]any, error) {
	var m map[string]any
	if len(e.Payload) == 0 {
		return m, nil
	}
	err := json.Unmarshal(e.Payload, &m)
	return m, err
}

// EncodePacket encodes an outgoing packet into an encrypted binary WebSocket message.
func EncodePacket(msgType string, payload any) ([]byte, error) {
	code, ok := OutgoingByName[msgType]
	if !ok {
		return nil, fmt.Errorf("unknown outgoing message type: %s", msgType)
	}

	packetArray := []any{code, payload}
	jsonBytes, err := json.Marshal(packetArray)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal packet JSON: %w", err)
	}

	var flags byte = 0
	data := jsonBytes

	// Compress with raw DEFLATE if size >= 8192
	if len(data) >= 8192 {
		var b bytes.Buffer
		w, err := flate.NewWriter(&b, 3)
		if err == nil {
			_, _ = w.Write(data)
			_ = w.Close()
			data = b.Bytes()
			flags = CompressFlag
		}
	}

	// Random 32-bit seed
	var seedBytes [4]byte
	_, _ = rand.Read(seedBytes[:])
	seed := binary.LittleEndian.Uint32(seedBytes[:])

	out := make([]byte, 5+len(data))
	binary.LittleEndian.PutUint32(out[0:4], seed)
	out[4] = flags
	copy(out[5:], data)

	// Encrypt in-place from offset 4
	XORCipher(out[4:], seed)

	return out, nil
}

// DecodeFrame decodes an incoming binary WebSocket frame from the game server.
func DecodeFrame(raw []byte) ([]RawEvent, error) {
	if len(raw) < 5 {
		return nil, nil
	}

	buf := make([]byte, len(raw))
	copy(buf, raw)

	seed := binary.LittleEndian.Uint32(buf[0:4])
	body := buf[4:]
	XORCipher(body, seed)

	flags := body[0]
	content := body[1:]

	// Non-batch frame
	if (flags & BatchFlag) == 0 {
		payload := content
		if (flags & CompressFlag) != 0 {
			r := flate.NewReader(bytes.NewReader(payload))
			decompressed, err := io.ReadAll(r)
			_ = r.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to decompress frame: %w", err)
			}
			payload = decompressed
		}

		event, err := parseJSONPacket(payload)
		if err != nil || event == nil {
			return nil, err
		}
		return []RawEvent{*event}, nil
	}

	// Batch frame with length-prefixed sub-packets
	var results []RawEvent
	t := 0
	for t+4 <= len(content) {
		pktLen := int(binary.LittleEndian.Uint32(content[t : t+4]))
		t += 4
		if t+pktLen > len(content) {
			break
		}
		sub := content[t : t+pktLen]
		event := decodeSingleSubPacket(sub)
		if event != nil {
			results = append(results, *event)
		}
		t += pktLen
	}

	return results, nil
}

func decodeSingleSubPacket(buf []byte) *RawEvent {
	if len(buf) < 5 {
		return nil
	}
	tmp := make([]byte, len(buf))
	copy(tmp, buf)

	seed := binary.LittleEndian.Uint32(tmp[0:4])
	body := tmp[4:]
	XORCipher(body, seed)

	flags := body[0]
	payload := body[1:]

	if (flags & CompressFlag) != 0 {
		r := flate.NewReader(bytes.NewReader(payload))
		decompressed, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			return nil
		}
		payload = decompressed
	}

	ev, _ := parseJSONPacket(payload)
	return ev
}

func parseJSONPacket(payload []byte) (*RawEvent, error) {
	var rawArray []json.RawMessage
	if err := json.Unmarshal(payload, &rawArray); err != nil {
		return nil, err
	}
	if len(rawArray) != 2 {
		return nil, fmt.Errorf("expected 2 elements in packet array, got %d", len(rawArray))
	}

	var code int
	if err := json.Unmarshal(rawArray[0], &code); err != nil {
		return nil, err
	}

	typeName, ok := IncomingByCode[code]
	if !ok {
		typeName = fmt.Sprintf("unknown_%d", code)
	}

	return &RawEvent{
		Type:    typeName,
		Opcode:  code,
		Payload: rawArray[1],
	}, nil
}
