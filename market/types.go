package market

import "time"

// PriceData represents price information for a trading pair
type PriceData struct {
	Pair      string  `json:"currency_pair"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
}

// CandleData represents a single candlestick data point
type CandleData struct {
	Timestamp int64   `json:"t"`
	Open      float64 `json:"o"`
	High      float64 `json:"h"`
	Low       float64 `json:"l"`
	Close     float64 `json:"c"`
	Volume    float64 `json:"v"`
}

// TickerData represents ticker information for a trading pair
type TickerData struct {
	Pair         string  `json:"currency_pair"`
	Last         float64 `json:"last"`
	LowestAsk    float64 `json:"lowest_ask"`
	HighestBid   float64 `json:"highest_bid"`
	PercentChange float64 `json:"percent_change"`
	BaseVolume   float64 `json:"base_volume"`
	QuoteVolume  float64 `json:"quote_volume"`
	IsFrozen     int     `json:"is_frozen"`
	High24hr     float64 `json:"high_24hr"`
	Low24hr      float64 `json:"low_24hr"`
}

// OrderBook represents the order book for a trading pair
type OrderBook struct {
	Asks [][]float64 `json:"asks"`
	Bids [][]float64 `json:"bids"`
	Timestamp int64 `json:"timestamp"`
}

// MarketEvent represents a market data event
type MarketEvent struct {
	Type      string      `json:"type"`
	Pair      string      `json:"pair"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}