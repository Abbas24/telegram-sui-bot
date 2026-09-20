package sui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ClientInfo represents the structure of a client.
type ClientInfo struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Remark        string `json:"remark"`
	Enable        bool   `json:"enable"`
	Volume        int64  `json:"volume"`
	Expiry        int64  `json:"expiry"`
	TotalUpload   int64  `json:"up"`
	TotalDownload int64  `json:"down"`
	Inbounds      []int  `json:"inbounds"`
	CreatedAt     int64  `json:"createdAt"`
	OnlineAt      int64  `json:"onlineAt"`
}

// Inbound represents the structure of an inbound.
type Inbound struct {
	ID     int    `json:"id"`
	Remark string `json:"remark"`
	Tag    string `json:"tag"`
	Type   string `json:"type"`
}

// Client interacts with the SUI API.
type Client struct {
	BaseURL string
	Token   string
}

// NewClient creates a new SUI client.
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
	}
}

// GetSettings fetches panel settings to get the base subscription URL.
func (c *Client) GetSettings() (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/apiv2/settings", c.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Token", c.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var response struct {
		Obj map[string]interface{} `json:"obj"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode settings: %w", err)
	}
	return response.Obj, nil
}

// GetClients returns the list of active clients.
func (c *Client) GetClients() ([]ClientInfo, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/apiv2/clients", c.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Token", c.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var allResponse struct {
		Obj struct {
			Clients []ClientInfo `json:"clients"`
		} `json:"obj"`
	}

	if err := json.Unmarshal(body, &allResponse); err != nil {
		return nil, fmt.Errorf("failed to decode clients: %w", err)
	}
	return allResponse.Obj.Clients, nil
}

// GetInbounds returns the list of inbounds.
func (c *Client) GetInbounds() ([]Inbound, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/apiv2/inbounds", c.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Token", c.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var allResponse struct {
		Obj struct {
			Inbounds []Inbound `json:"inbounds"`
		} `json:"obj"`
	}

	if err := json.Unmarshal(body, &allResponse); err != nil {
		return nil, fmt.Errorf("failed to decode inbounds: %w", err)
	}
	return allResponse.Obj.Inbounds, nil
}

// CreateClient creates a new client.
func (c *Client) CreateClient(name string, inbounds []int, volumeGB int) (*ClientInfo, error) {
	expiry := time.Now().AddDate(0, 0, 30).Unix()
	volumeBytes := int64(volumeGB) * 1024 * 1024 * 1024

	config := map[string]interface{}{"type": "vmess"}
	data := map[string]interface{}{
		"object":   "clients",
		"action":   "new",
		"name":     name,
		"remark":   name,
		"enable":   true,
		"inbounds": inbounds,
		"volume":   volumeBytes,
		"expiry":   expiry,
		"config":   config,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/apiv2/save", c.BaseURL), bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Token", c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	clients, err := c.GetClients()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch client list: %w", err)
	}

	for _, client := range clients {
		if client.Name == name {
			return &client, nil
		}
	}

	return nil, fmt.Errorf("client not found in list after creation")
}
