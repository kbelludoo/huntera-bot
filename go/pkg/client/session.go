package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const BaseURL = "https://huntera.com.br"
const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

type Character struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

type GameTicket struct {
	Ticket       string    `json:"ticket"`
	WebsocketURL string    `json:"websocketUrl"`
	Character    Character `json:"character"`
}

type SessionResult struct {
	Cookie  string
	Account map[string]any
}

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

func LoginWithCredentials(email, password string) (*SessionResult, error) {
	bodyMap := map[string]string{
		"email":    email,
		"password": password,
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	req, err := http.NewRequest("POST", BaseURL+"/api/auth/login", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read login response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("login failed with HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	// Extract tw_session cookie
	var twSession string
	twSessionRegex := regexp.MustCompile(`tw_session=([^;]+)`)
	for _, cookie := range resp.Header.Values("Set-Cookie") {
		if match := twSessionRegex.FindStringSubmatch(cookie); len(match) > 1 {
			twSession = "tw_session=" + match[1]
			break
		}
	}

	if twSession == "" && len(resp.Header.Values("Set-Cookie")) > 0 {
		var parts []string
		for _, c := range resp.Header.Values("Set-Cookie") {
			parts = append(parts, strings.Split(c, ";")[0])
		}
		twSession = strings.Join(parts, "; ")
	}

	var parsed map[string]any
	_ = json.Unmarshal(respBytes, &parsed)

	return &SessionResult{
		Cookie:  twSession,
		Account: parsed,
	}, nil
}

func GetCharacters(cookie string) ([]Character, error) {
	req, err := http.NewRequest("GET", BaseURL+"/api/characters", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get characters failed with HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	// Can be []Character or {"characters": []Character}
	var chars []Character
	if err := json.Unmarshal(respBytes, &chars); err == nil && len(chars) > 0 {
		return chars, nil
	}

	var wrapped struct {
		Characters []Character `json:"characters"`
	}
	if err := json.Unmarshal(respBytes, &wrapped); err == nil {
		return wrapped.Characters, nil
	}

	return nil, fmt.Errorf("failed to parse characters response: %s", string(respBytes))
}

func GetGameTicket(charID string, cookie string) (*GameTicket, error) {
	bodyMap := map[string]string{
		"characterId": charID,
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	req, err := http.NewRequest("POST", BaseURL+"/api/game-tickets", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("get game ticket failed with HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var ticket GameTicket
	if err := json.Unmarshal(respBytes, &ticket); err != nil {
		return nil, fmt.Errorf("failed to parse game ticket: %w", err)
	}

	if ticket.WebsocketURL == "" {
		ticket.WebsocketURL = "wss://huntera.com.br/game-socket"
	}

	return &ticket, nil
}
