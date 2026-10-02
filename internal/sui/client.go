package sui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

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

type Inbound struct {
	ID         int                    `json:"id"`
	Remark     string                 `json:"remark"`
	Tag        string                 `json:"tag"`
	Type       string                 `json:"type"`
	Listen     string                 `json:"listen"`
	ListenPort int                    `json:"listen_port"`
	TlsID      int                    `json:"tls_id"`
	Addrs      []map[string]interface{} `json:"addrs"`
	OutJson    map[string]interface{} `json:"out_json"`
	Extra      map[string]interface{} `json:"-"`
}

type Client struct {
	BaseURL    string
	Token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	req.Header.Set("Token", c.Token)
	return c.httpClient.Do(req)
}

func (c *Client) getJSON(endpoint string, target interface{}) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/apiv2/%s", strings.TrimRight(c.BaseURL, "/"), endpoint), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if len(body) == 0 {
		return fmt.Errorf("empty response body from SUI API")
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode response: %w, body: %s", err, string(body))
	}
	return nil
}

func (c *Client) postJSON(endpoint string, payload interface{}, target interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	log.Printf("SUI API Request: %s/apiv2/%s payload: %s", c.BaseURL, endpoint, string(jsonData))

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/apiv2/%s", strings.TrimRight(c.BaseURL, "/"), endpoint), bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	log.Printf("SUI API Response: %s", string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if len(body) == 0 {
		return fmt.Errorf("empty response body from SUI API")
	}

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode response: %w, body: %s", err, string(body))
	}
	return nil
}

func (c *Client) GetSettings() (map[string]interface{}, error) {
	var response struct {
		Success bool                   `json:"success"`
		Msg     string                 `json:"msg"`
		Obj     map[string]interface{} `json:"obj"`
	}
	if err := c.getJSON("settings", &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("API error: %s", response.Msg)
	}
	return response.Obj, nil
}

func (c *Client) GetClients() ([]ClientInfo, error) {
	var response struct {
		Success bool `json:"success"`
		Msg     string `json:"msg"`
		Obj     struct {
			Clients []ClientInfo `json:"clients"`
		} `json:"obj"`
	}
	if err := c.getJSON("clients", &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("API error: %s", response.Msg)
	}
	return response.Obj.Clients, nil
}

func (c *Client) GetInbounds() ([]Inbound, error) {
	var response struct {
		Success bool `json:"success"`
		Msg     string `json:"msg"`
		Obj     struct {
			Inbounds []Inbound `json:"inbounds"`
		} `json:"obj"`
	}
	if err := c.getJSON("inbounds", &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("API error: %s", response.Msg)
	}
	return response.Obj.Inbounds, nil
}

func (c *Client) SaveInbound(action string, inbound map[string]interface{}) (map[string]interface{}, error) {
	var response struct {
		Success bool                   `json:"success"`
		Msg     string                 `json:"msg"`
		Obj     map[string]interface{} `json:"obj"`
	}
	payload := map[string]interface{}{
		"object": "inbounds",
		"action": action,
		"data":   inbound,
	}
	if err := c.postJSON("save", payload, &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("API error: %s", response.Msg)
	}
	return response.Obj, nil
}

func (c *Client) CreateClient(name string, inbounds []int, volumeGB int, days int, configType string) (*ClientInfo, error) {
	expiry := time.Now().AddDate(0, 0, days).Unix()
	volumeBytes := int64(volumeGB) * 1024 * 1024 * 1024

	if configType == "" {
		configType = "vmess"
	}

	config := map[string]interface{}{"type": configType}
	data := map[string]interface{}{
		"enable":   true,
		"name":     name,
		"remark":   name,
		"inbounds": inbounds,
		"volume":   volumeBytes,
		"expiry":   expiry,
		"config":   config,
	}

	var response struct {
		Success bool `json:"success"`
		Msg     string `json:"msg"`
		Obj     struct {
			Clients []ClientInfo `json:"clients"`
		} `json:"obj"`
	}
	payload := map[string]interface{}{
		"object": "clients",
		"action": "new",
		"data":   data,
	}
	if err := c.postJSON("save", payload, &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("API error: %s", response.Msg)
	}

	var latestClient *ClientInfo
	for i := range response.Obj.Clients {
		if response.Obj.Clients[i].Name == name {
			if latestClient == nil || response.Obj.Clients[i].CreatedAt > latestClient.CreatedAt {
				latestClient = &response.Obj.Clients[i]
			}
		}
	}

	if latestClient != nil {
		return latestClient, nil
	}

	return nil, fmt.Errorf("client created but not found in response")
}

func (c *Client) UpdateClient(clientID int, updates map[string]interface{}) (*ClientInfo, error) {
	clients, err := c.GetClients()
	if err != nil {
		return nil, err
	}

	var targetClient *ClientInfo
	for i := range clients {
		if clients[i].ID == clientID {
			targetClient = &clients[i]
			break
		}
	}

	if targetClient == nil {
		return nil, fmt.Errorf("client not found")
	}

	// Marshaling to map[string]interface{} to apply updates
	clientData, err := json.Marshal(targetClient)
	if err != nil {
		return nil, fmt.Errorf("marshal client: %w", err)
	}
	var fullObject map[string]interface{}
	if err := json.Unmarshal(clientData, &fullObject); err != nil {
		return nil, fmt.Errorf("unmarshal client: %w", err)
	}

	// Apply updates
	for k, v := range updates {
		fullObject[k] = v
	}

	var response struct {
		Success bool `json:"success"`
		Msg     string `json:"msg"`
		Obj     struct {
			Clients []ClientInfo `json:"clients"`
		} `json:"obj"`
	}
	payload := map[string]interface{}{
		"object": "clients",
		"action": "edit",
		"data":   fullObject, // Sending full object as required by wiki
	}
	if err := c.postJSON("save", payload, &response); err != nil {
		return nil, err
	}
	if !response.Success {
		return nil, fmt.Errorf("API error: %s", response.Msg)
	}

	// Return the updated client from response
	for _, client := range response.Obj.Clients {
		if client.ID == clientID {
			return &client, nil
		}
	}
	return nil, fmt.Errorf("client updated but not found in response")
}

func (c *Client) DeleteClient(clientID int) error {
	data := map[string]interface{}{
		"id": clientID,
	}
	var response struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	payload := map[string]interface{}{
		"object": "clients",
		"action": "del",
		"data":   data,
	}
	if err := c.postJSON("save", payload, &response); err != nil {
		return err
	}
	if !response.Success {
		return fmt.Errorf("API error: %s", response.Msg)
	}
	return nil
}
