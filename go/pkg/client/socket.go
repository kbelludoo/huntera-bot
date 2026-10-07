package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"huntera-bot/pkg/protocol"

	"github.com/gorilla/websocket"
)

type EventHandler func(event protocol.RawEvent)

type SocketClient struct {
	cookie        string
	wsURL         string
	characterName string
	ticket        string

	conn      *websocket.Conn
	writeMu   sync.Mutex
	closeChan chan struct{}

	handlersMu sync.RWMutex
	handlers   map[string][]EventHandler

	latency   time.Duration
	connected bool
}

func NewSocketClient(cookie string) *SocketClient {
	return &SocketClient{
		cookie:    cookie,
		handlers:  make(map[string][]EventHandler),
		closeChan: make(chan struct{}),
	}
}

func (s *SocketClient) On(eventType string, handler EventHandler) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.handlers[eventType] = append(s.handlers[eventType], handler)
}

func (s *SocketClient) dispatch(event protocol.RawEvent) {
	s.handlersMu.RLock()
	handlers := s.handlers[event.Type]
	allHandlers := s.handlers["*"]
	s.handlersMu.RUnlock()

	for _, h := range handlers {
		h(event)
	}
	for _, h := range allHandlers {
		h(event)
	}
}

func (s *SocketClient) Connect(charName, ticket, wsURL string) error {
	s.characterName = charName
	s.ticket = ticket
	s.wsURL = wsURL
	if s.wsURL == "" {
		s.wsURL = "wss://huntera.com.br/game-socket"
	}

	headers := http.Header{}
	headers.Set("User-Agent", UserAgent)
	headers.Set("Origin", "https://huntera.com.br")
	if s.cookie != "" {
		headers.Set("Cookie", s.cookie)
	}

	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: false},
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(s.wsURL, headers)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	s.conn = conn
	s.connected = true
	s.closeChan = make(chan struct{})

	// 1. Send authentication packet
	authPayload := map[string]any{
		"clientVersion": protocol.ClientVersion,
		"ticket":        s.ticket,
	}
	if err := s.Send("authenticate", authPayload); err != nil {
		_ = s.conn.Close()
		return fmt.Errorf("failed to send authenticate: %w", err)
	}

	// 2. Start ping loop (every 5 seconds)
	go s.pingLoop()

	// 3. Start read loop
	go s.readLoop()

	s.dispatch(protocol.RawEvent{Type: "connected"})
	return nil
}

func (s *SocketClient) Send(msgType string, payload any) error {
	if !s.connected || s.conn == nil {
		return fmt.Errorf("not connected")
	}

	encoded, err := protocol.EncodePacket(msgType, payload)
	if err != nil {
		return fmt.Errorf("failed to encode packet: %w", err)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.BinaryMessage, encoded)
}

func (s *SocketClient) pingLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.closeChan:
			return
		case t := <-ticker.C:
			if !s.connected {
				return
			}
			_ = s.Send("ping", map[string]any{"t": t.UnixMilli()})
		}
	}
}

func (s *SocketClient) readLoop() {
	defer func() {
		s.connected = false
		s.dispatch(protocol.RawEvent{Type: "disconnected"})
	}()

	for {
		messageType, data, err := s.conn.ReadMessage()
		if err != nil {
			return
		}

		if messageType == websocket.BinaryMessage {
			events, err := protocol.DecodeFrame(data)
			if err != nil {
				continue
			}

			for _, ev := range events {
				if ev.Type == "pong" {
					var pongData struct {
						T int64 `json:"t"`
					}
					if err := ev.Unmarshal(&pongData); err == nil && pongData.T > 0 {
						s.latency = time.Duration(time.Now().UnixMilli()-pongData.T) * time.Millisecond
					}
					continue
				}
				s.dispatch(ev)
			}
		}
	}
}

func (s *SocketClient) Close() {
	if !s.connected {
		return
	}
	s.connected = false
	close(s.closeChan)
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *SocketClient) Latency() time.Duration {
	return s.latency
}

// Convenience Game Actions
func (s *SocketClient) StartHunt(huntID string, tier int) error {
	return s.Send("start-hunt", map[string]any{"huntId": huntID, "tier": tier})
}

func (s *SocketClient) ChangeHunt(huntID string, tier int) error {
	return s.Send("change-hunt", map[string]any{"huntId": huntID, "tier": tier})
}

func (s *SocketClient) LeaveHunt() error {
	return s.Send("leave-hunt", map[string]any{})
}

func (s *SocketClient) UnlockBestiary(monsterID string) error {
	return s.Send("bestiary-unlock", map[string]any{"monsterId": monsterID})
}

func (s *SocketClient) BuyStamina(cost int) error {
	return s.Send("buy-stamina", map[string]any{"cost": cost})
}

func (s *SocketClient) SetIdleTraining(enabled bool) error {
	return s.Send("set-idle-training", map[string]any{"enabled": enabled})
}

func (s *SocketClient) RequestCyclopedia() error {
	return s.Send("cyclopedia-request", map[string]any{})
}
