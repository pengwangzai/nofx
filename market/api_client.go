package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// APIClient represents a client for interacting with exchange APIs
type APIClient struct {
	BaseURL    string
	APIKey     string
	SecretKey  string
	HTTPClient *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL, apiKey, secretKey string) *APIClient {
	return &APIClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		SecretKey: secretKey,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetPrice gets the current price for a trading pair
func (c *APIClient) GetPrice(pair string) (*PriceData, error) {
	url := fmt.Sprintf("%s/market/price?currency_pair=%s", c.BaseURL, pair)
	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var price PriceData
	if err := json.Unmarshal(body, &price); err != nil {
		return nil, err
	}

	return &price, nil
}

// GetCandles gets historical price data (candles) for a trading pair
func (c *APIClient) GetCandles(pair, interval string, limit int) ([]CandleData, error) {
	url := fmt.Sprintf("%s/market/candles?currency_pair=%s\u0026interval=%s\u0026limit=%d",
		c.BaseURL, pair, interval, limit)
	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var candles []CandleData
	if err := json.Unmarshal(body, &candles); err != nil {
		return nil, err
	}

	return candles, nil
}

// doRequest performs an HTTP request with authentication
func (c *APIClient) doRequest(method, url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	// Add authentication headers
	if c.APIKey != "" {
		req.Header.Set("KEY", c.APIKey)
		// Add signature for authenticated requests
		// This is a placeholder for actual signature implementation
	}

	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}