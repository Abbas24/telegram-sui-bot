package sui

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Client for S-UI
type Client struct {
	BaseURL string
	Token   string
}

// ClientInfo represents active client
type ClientInfo struct {
	Name          string `json:"name"`
	TotalUpload   int64  `json:"up"`
	TotalDownload int64  `json:"down"`
}

// ApiResponse represents the actual response structure
type ApiResponse struct {
	Success bool   `json:"success"`
	Message string `json:"msg"`
	// The API returns an object with a "clients" field, which is a list of ClientInfo
	Obj struct {
		Clients []ClientInfo `json:"clients"`
	} `json:"obj"`
}

// NewClient creates a new S-UI client
func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
	}
}

// GetClients fetches clients
func (c *Client) GetClients() ([]ClientInfo, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/apiv2/clients", c.BaseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Token", c.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp ApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		// Log the error to see what's actually happening
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Message)
	}

	return apiResp.Obj.Clients, nil
}
