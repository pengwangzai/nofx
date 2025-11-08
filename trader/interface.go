package trader

// OrderType represents the type of order
type OrderType string

const (
	// MarketOrder is an order executed immediately at current market price
	MarketOrder OrderType = "market"
	// LimitOrder is an order to be executed at a specific price or better
	LimitOrder OrderType = "limit"
	// StopOrder is an order to buy/sell when price reaches a specified level
	StopOrder OrderType = "stop"
	// StopLimitOrder is a stop order that becomes a limit order when triggered
	StopLimitOrder OrderType = "stop_limit"
)

// Side represents the side of the order
type Side string

const (
	// BuySide represents a buy order
	BuySide Side = "buy"
	// SellSide represents a sell order
	SellSide Side = "sell"
)

// Status represents the status of an order
type Status string

const (
	// OrderStatusNew is a newly created order
	OrderStatusNew Status = "new"
	// OrderStatusPartiallyFilled is a partially filled order
	OrderStatusPartiallyFilled Status = "partially_filled"
	// OrderStatusFilled is a completely filled order
	OrderStatusFilled Status = "filled"
	// OrderStatusCanceled is a canceled order
	OrderStatusCanceled Status = "canceled"
	// OrderStatusRejected is a rejected order
	OrderStatusRejected Status = "rejected"
	// OrderStatusExpired is an expired order
	OrderStatusExpired Status = "expired"
)

// Order represents a trading order
type Order struct {
	ID            string    `json:"id"`
	ClientOrderID string    `json:"client_order_id"`
	Pair          string    `json:"currency_pair"`
	Type          OrderType `json:"type"`
	Side          Side      `json:"side"`
	Price         float64   `json:"price"`
	Amount        float64   `json:"amount"`
	FilledAmount  float64   `json:"filled_amount"`
	Status        Status    `json:"status"`
	TimeInForce   string    `json:"time_in_force"`
	CreatedTime   int64     `json:"created_time"`
	UpdatedTime   int64     `json:"updated_time"`
}

// Position represents a trading position
type Position struct {
	ID           string  `json:"id"`
	Pair         string  `json:"currency_pair"`
	Side         Side    `json:"side"`
	Size         float64 `json:"size"`
	EntryPrice   float64 `json:"entry_price"`
	MarkPrice    float64 `json:"mark_price"`
	UnrealizedPnl float64 `json:"unrealized_pnl"`
	RealizedPnl  float64 `json:"realized_pnl"`
	Leverage     int64   `json:"leverage"`
	LiquidationPrice float64 `json:"liquidation_price"`
	Status       string  `json:"status"`
	CreatedTime  int64   `json:"created_time"`
	UpdatedTime  int64   `json:"updated_time"`
}

// Balance represents account balance
type Balance struct {
	Currency     string  `json:"currency"`
	Total        float64 `json:"total"`
	Available    float64 `json:"available"`
	InOrders     float64 `json:"in_orders"`
	Staked       float64 `json:"staked,omitempty"`
}

// Trader interface defines methods for interacting with trading exchanges
type Trader interface {
	// GetBalance retrieves the account balance
	GetBalance() ([]Balance, error)

	// GetPosition retrieves the current position for a trading pair
	GetPosition(pair string) (*Position, error)

	// GetPositions retrieves all current positions
	GetPositions() ([]Position, error)

	// CreateOrder creates a new order
	CreateOrder(pair string, side Side, orderType OrderType, amount, price float64, leverage int64) (*Order, error)

	// CancelOrder cancels an existing order
	CancelOrder(orderID string) error

	// GetOrder retrieves an order by ID
	GetOrder(orderID string) (*Order, error)

	// GetOrders retrieves all orders
	GetOrders(pair string, status Status) ([]Order, error)

	// ClosePosition closes an open position
	ClosePosition(pair string, amount float64) (*Order, error)

	// SetLeverage sets the leverage for a trading pair
	SetLeverage(pair string, leverage int64) error
}