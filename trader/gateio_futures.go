package trader

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	gateapi "github.com/gateio/gateapi-go/v6"
	"github.com/antihax/optional"
)

// GateIOFuturesTrader Gate.io合约交易器
type GateIOFuturesTrader struct {
	client *gateapi.APIClient
	ctx    context.Context

	// 余额缓存
	cachedBalance     map[string]interface{}
	balanceCacheTime  time.Time
	balanceCacheMutex sync.RWMutex

	// 持仓缓存
	cachedPositions     []map[string]interface{}
	positionsCacheTime  time.Time
	positionsCacheMutex sync.RWMutex

	// 交易对信息缓存（用于精度等）
	symbolInfoCache     map[string]*gateapi.Contract
	symbolInfoCacheTime time.Time
	symbolInfoMutex     sync.RWMutex

	// 缓存有效期（15秒）
	cacheDuration time.Duration
}

// NewGateIOFuturesTrader 创建Gate.io合约交易器
func NewGateIOFuturesTrader(apiKey, secretKey string) *GateIOFuturesTrader {
	// 验证API Key和Secret Key
	if apiKey == "" {
		log.Printf("⚠️ Gate.io API Key为空，请检查配置")
	}
	if secretKey == "" {
		log.Printf("⚠️ Gate.io Secret Key为空，请检查配置")
	}
	if apiKey == "" || secretKey == "" {
		log.Printf("⚠️ Gate.io API Key或Secret Key为空，交易器可能无法正常工作")
	}

	// 创建 Gate.io API 客户端配置
	config := gateapi.NewConfiguration()
	config.BasePath = "https://api.gateio.ws/api/v4"

	// 创建 API 客户端
	client := gateapi.NewAPIClient(config)

	// 创建认证上下文
	ctx := context.WithValue(
		context.Background(),
		gateapi.ContextGateAPIV4,
		gateapi.GateAPIV4{
			Key:    apiKey,
			Secret: secretKey,
		},
	)

	trader := &GateIOFuturesTrader{
		client:          client,
		ctx:             ctx,
		cacheDuration:   15 * time.Second,
		symbolInfoCache: make(map[string]*gateapi.Contract),
	}

	// 显示API Key前8位用于调试（不显示完整密钥）
	apiKeyPrefix := ""
	if len(apiKey) > 8 {
		apiKeyPrefix = apiKey[:8] + "..."
	} else if len(apiKey) > 0 {
		apiKeyPrefix = apiKey[:len(apiKey)] + "..."
	} else {
		apiKeyPrefix = "(空)"
	}
	log.Printf("✓ Gate.io合约交易器初始化成功 (API Key: %s)", apiKeyPrefix)
	return trader
}

// normalizeSymbolForGateIO 标准化交易对符号为Gate.io格式
// Binance格式: BTCUSDT -> Gate.io格式: BTC_USDT
func normalizeSymbolForGateIO(symbol string) string {
	// 如果已经是Gate.io格式（包含下划线），直接返回
	if strings.Contains(symbol, "_") {
		return symbol
	}

	// 从Binance格式转换：BTCUSDT -> BTC_USDT
	suffixes := []string{"USDT", "USDC", "BUSD", "TUSD", "DAI", "USD"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(symbol, suffix) {
			base := strings.TrimSuffix(symbol, suffix)
			return base + "_" + suffix
		}
	}

	// 如果找不到已知后缀，尝试在最后4个字符前插入下划线
	if len(symbol) > 4 {
		return symbol[:len(symbol)-4] + "_" + symbol[len(symbol)-4:]
	}

	return symbol
}

// DenormalizeSymbolFromGateIO 反标准化交易对符号
// Gate.io格式: BTC_USDT -> Binance格式: BTCUSDT
func DenormalizeSymbolFromGateIO(symbol string) string {
	// 如果已经是Binance格式（不包含下划线），直接返回
	if !strings.Contains(symbol, "_") {
		return symbol
	}

	// 从Gate.io格式转换：BTC_USDT -> BTCUSDT
	return strings.ReplaceAll(symbol, "_", "")
}

// GetBalance 获取账户余额（带缓存）
func (t *GateIOFuturesTrader) GetBalance() (map[string]interface{}, error) {
	// 先检查缓存是否有效
	t.balanceCacheMutex.RLock()
	if t.cachedBalance != nil && time.Since(t.balanceCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.balanceCacheTime)
		t.balanceCacheMutex.RUnlock()
		log.Printf("✓ 使用缓存的账户余额（缓存时间: %.1f秒前）", cacheAge.Seconds())
		return t.cachedBalance, nil
	}
	t.balanceCacheMutex.RUnlock()

	// 缓存过期或不存在，调用API
	log.Printf("🔄 缓存过期，正在调用Gate.io API获取账户余额...")

	// 使用 SDK 获取账户余额
	account, _, err := t.client.FuturesApi.ListFuturesAccounts(t.ctx, "usdt")
	if err != nil {
		log.Printf("❌ Gate.io API调用失败: %v", err)
		return nil, fmt.Errorf("获取账户信息失败: %w", err)
	}

	totalBalance, _ := strconv.ParseFloat(account.Total, 64)
	availableBalance, _ := strconv.ParseFloat(account.Available, 64)
	unrealizedPnL, _ := strconv.ParseFloat(account.UnrealisedPnl, 64)

	result := make(map[string]interface{})
	result["totalWalletBalance"] = totalBalance
	result["availableBalance"] = availableBalance
	result["totalUnrealizedProfit"] = unrealizedPnL

	log.Printf("✓ Gate.io API返回: 总余额=%.2f, 可用=%.2f, 未实现盈亏=%.2f",
		totalBalance, availableBalance, unrealizedPnL)

	// 更新缓存
	t.balanceCacheMutex.Lock()
	t.cachedBalance = result
	t.balanceCacheTime = time.Now()
	t.balanceCacheMutex.Unlock()

	return result, nil
}

// GetPositions 获取所有持仓（带缓存）
func (t *GateIOFuturesTrader) GetPositions() ([]map[string]interface{}, error) {
	// 先检查缓存是否有效
	t.positionsCacheMutex.RLock()
	if t.cachedPositions != nil && time.Since(t.positionsCacheTime) < t.cacheDuration {
		cacheAge := time.Since(t.positionsCacheTime)
		t.positionsCacheMutex.RUnlock()
		log.Printf("✓ 使用缓存的持仓信息（缓存时间: %.1f秒前）", cacheAge.Seconds())
		return t.cachedPositions, nil
	}
	t.positionsCacheMutex.RUnlock()

	// 缓存过期或不存在，调用API
	log.Printf("🔄 缓存过期，正在调用Gate.io API获取持仓信息...")

	// 使用 SDK 获取持仓
	positions, _, err := t.client.FuturesApi.ListPositions(t.ctx, "usdt", nil)
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var result []map[string]interface{}
	for _, pos := range positions {
		// Size字段是int64类型，表示合约数量，需要转换为float64
		contractSize := float64(pos.Size)
		if contractSize == 0 {
			continue // 跳过无持仓的
		}

		// 转换符号格式：Gate.io格式(BTC_USDT) -> Binance格式(BTCUSDT)
		binanceSymbol := DenormalizeSymbolFromGateIO(pos.Contract)

		// 将合约数量转换为币种数量（乘以 quanto_multiplier）
		// 注意：需要获取合约信息来获取 quanto_multiplier
		coinQuantity := contractSize // 默认值，如果无法获取合约信息则使用合约数量
		info, err := t.getSymbolInfo(binanceSymbol)
		if err == nil && info.QuantoMultiplier != "" {
			quantoMultiplier, parseErr := strconv.ParseFloat(info.QuantoMultiplier, 64)
			if parseErr == nil && quantoMultiplier > 0 {
				coinQuantity = contractSize * quantoMultiplier
			} else {
				log.Printf("  ⚠ 无法解析 %s 的 quanto_multiplier (%s)，使用合约数量作为币种数量", binanceSymbol, info.QuantoMultiplier)
			}
		} else {
			// 如果无法获取合约信息，记录警告但继续处理
			log.Printf("  ⚠ 无法获取 %s 的 quanto_multiplier，使用合约数量作为币种数量", binanceSymbol)
		}

		posMap := make(map[string]interface{})
		posMap["symbol"] = binanceSymbol // 使用Binance格式，保持与系统其他部分一致
		posMap["positionAmt"] = coinQuantity // 币种数量
		posMap["entryPrice"], _ = strconv.ParseFloat(pos.EntryPrice, 64)
		posMap["markPrice"], _ = strconv.ParseFloat(pos.MarkPrice, 64)
		posMap["unRealizedProfit"], _ = strconv.ParseFloat(pos.UnrealisedPnl, 64)
		posMap["leverage"], _ = strconv.ParseFloat(pos.Leverage, 64)
		posMap["liquidationPrice"], _ = strconv.ParseFloat(pos.LiqPrice, 64)

		// 判断方向（Gate.io中正数为多仓，负数为空仓）
		if contractSize > 0 {
			posMap["side"] = "long"
		} else {
			posMap["side"] = "short"
			posMap["positionAmt"] = -coinQuantity // 转为正数（币种数量）
		}

		result = append(result, posMap)
	}

	// 更新缓存
	t.positionsCacheMutex.Lock()
	t.cachedPositions = result
	t.positionsCacheTime = time.Now()
	t.positionsCacheMutex.Unlock()

	return result, nil
}

// getSymbolInfo 获取交易对信息（带缓存）
func (t *GateIOFuturesTrader) getSymbolInfo(symbol string) (*gateapi.Contract, error) {
	// 转换符号格式（Gate.io使用BTC_USDT格式）
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	// 先检查缓存（使用Gate.io格式的symbol作为key）
	t.symbolInfoMutex.RLock()
	if info, exists := t.symbolInfoCache[gateIOSymbol]; exists {
		if time.Since(t.symbolInfoCacheTime) < 5*time.Minute { // 交易对信息缓存5分钟
			t.symbolInfoMutex.RUnlock()
			return info, nil
		}
	}
	t.symbolInfoMutex.RUnlock()

	// 获取所有交易对信息
	contracts, _, err := t.client.FuturesApi.ListFuturesContracts(t.ctx, "usdt", nil)
	if err != nil {
		return nil, fmt.Errorf("获取交易对信息失败: %w", err)
	}

	// 更新缓存（使用Gate.io格式的symbol作为key）
	t.symbolInfoMutex.Lock()
	t.symbolInfoCache = make(map[string]*gateapi.Contract)
	for i := range contracts {
		contract := contracts[i]
		t.symbolInfoCache[contract.Name] = &contract
	}
	t.symbolInfoCacheTime = time.Now()
	t.symbolInfoMutex.Unlock()

	// 查找指定交易对（使用Gate.io格式）
	if info, exists := t.symbolInfoCache[gateIOSymbol]; exists {
		return info, nil
	}

	return nil, fmt.Errorf("未找到交易对: %s (Gate.io格式: %s)", symbol, gateIOSymbol)
}

// convertCoinQuantityToContractSize 将币种数量转换为合约数量
// quantity: 币种数量（如 BTC 的数量）
// 返回: 合约数量（需要除以 quanto_multiplier）
func (t *GateIOFuturesTrader) convertCoinQuantityToContractSize(symbol string, coinQuantity float64) (float64, error) {
	info, err := t.getSymbolInfo(symbol)
	if err != nil {
		return 0, fmt.Errorf("获取合约信息失败: %w", err)
	}

	// 获取 quanto_multiplier（每张合约对应的币种数量）
	// 注意：QuantoMultiplier 是字符串类型，需要解析为数字
	quantoMultiplier := 1.0 // 默认值
	if info.QuantoMultiplier != "" {
		parsed, err := strconv.ParseFloat(info.QuantoMultiplier, 64)
		if err == nil && parsed > 0 {
			quantoMultiplier = parsed
		} else {
			log.Printf("  ⚠ %s 的 quanto_multiplier (%s) 解析失败或无效，假设为 1", symbol, info.QuantoMultiplier)
		}
	} else {
		log.Printf("  ⚠ %s 的 quanto_multiplier 为空，假设为 1", symbol)
	}

	// 将币种数量转换为合约数量
	contractSize := coinQuantity / quantoMultiplier

	return contractSize, nil
}

// FormatQuantity 格式化合约数量到正确的精度
// quantity: 币种数量（输入），函数内部会转换为合约数量
// 返回: 格式化后的合约数量字符串
func (t *GateIOFuturesTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	info, err := t.getSymbolInfo(symbol)
	if err != nil {
		// 如果获取失败，使用默认精度3
		log.Printf("  ⚠ %s 未找到精度信息，使用默认精度3", symbol)
		return fmt.Sprintf("%.3f", quantity), nil
	}

	// 将币种数量转换为合约数量
	contractSize, err := t.convertCoinQuantityToContractSize(symbol, quantity)
	if err != nil {
		return "", fmt.Errorf("转换币种数量到合约数量失败: %w", err)
	}

	// 从合约信息中获取精度（根据 OrderPriceRound 推断，或使用默认值）
	precision := 3 // 默认精度
	if info.OrderPriceRound != "" {
		// 尝试从 OrderPriceRound 推断精度（例如 "0.01" -> 2位小数）
		if strings.Contains(info.OrderPriceRound, ".") {
			parts := strings.Split(info.OrderPriceRound, ".")
			if len(parts) == 2 {
				precision = len(parts[1])
			}
		}
	}

	// 检查最小订单数量（OrderSizeMin 是合约的最小数量）
	if info.OrderSizeMin > 0 {
		minContractSize := float64(info.OrderSizeMin)
		if contractSize < minContractSize {
			// 获取当前价格，计算最小开仓金额
			price, priceErr := t.GetMarketPrice(symbol)
			var minNotionalMsg string
			if priceErr == nil && price > 0 {
				// 计算最小币种数量
				quantoMultiplier := 1.0
				if info.QuantoMultiplier != "" {
					parsed, err := strconv.ParseFloat(info.QuantoMultiplier, 64)
					if err == nil && parsed > 0 {
						quantoMultiplier = parsed
					}
				}
				minCoinQuantity := minContractSize * quantoMultiplier
				minNotional := minCoinQuantity * price
				minNotionalMsg = fmt.Sprintf("最小开仓金额: %.2f USDT (最小合约数量: %.8f, 对应币种数量: %.8f × 价格: %.2f)",
					minNotional, minContractSize, minCoinQuantity, price)
			} else {
				minNotionalMsg = fmt.Sprintf("最小合约数量: %.8f", minContractSize)
			}
			return "", fmt.Errorf("订单数量 %.8f (合约数量: %.8f) 小于最小要求 %.8f。%s。建议增加开仓金额",
				quantity, contractSize, minContractSize, minNotionalMsg)
		}
	}

	// 格式化合约数量
	format := fmt.Sprintf("%%.%df", precision)
	formatted := fmt.Sprintf(format, contractSize)

	// 检查格式化后的数量是否为0
	formattedFloat, parseErr := strconv.ParseFloat(formatted, 64)
	if parseErr != nil || formattedFloat <= 0 {
		// 获取当前价格，计算最小开仓金额
		price, priceErr := t.GetMarketPrice(symbol)
		var suggestionMsg string
		if priceErr == nil && price > 0 {
			// 计算需要的最小数量（基于精度）
			minContractQuantity := 1.0 / math.Pow10(precision)
			quantoMultiplier := 1.0
			if info.QuantoMultiplier != "" {
				parsed, err := strconv.ParseFloat(info.QuantoMultiplier, 64)
				if err == nil && parsed > 0 {
					quantoMultiplier = parsed
				}
			}
			minCoinQuantity := minContractQuantity * quantoMultiplier
			minNotional := minCoinQuantity * price
			suggestionMsg = fmt.Sprintf("由于精度限制（%d位小数），最小合约数量为 %.8f，对应币种数量为 %.8f，最小开仓金额约为 %.2f USDT",
				precision, minContractQuantity, minCoinQuantity, minNotional)
		} else {
			suggestionMsg = fmt.Sprintf("由于精度限制（%d位小数），合约数量过小被截断为0", precision)
		}
		return "", fmt.Errorf("数量 %.8f (合约数量: %.8f) 格式化后为 0（精度: %d位小数）。%s。建议增加开仓金额",
			quantity, contractSize, precision, suggestionMsg)
	}

	return formatted, nil
}

// GetMarketPrice 获取市场价格
func (t *GateIOFuturesTrader) GetMarketPrice(symbol string) (float64, error) {
	// 转换符号格式
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	// 使用 SDK 获取 ticker
	opts := &gateapi.ListFuturesTickersOpts{}
	if gateIOSymbol != "" {
		opts.Contract = optional.NewString(gateIOSymbol)
	}
	tickers, _, err := t.client.FuturesApi.ListFuturesTickers(t.ctx, "usdt", opts)
	if err != nil {
		return 0, fmt.Errorf("获取价格失败: %w", err)
	}

	if len(tickers) == 0 {
		return 0, fmt.Errorf("未找到价格")
	}

	price, err := strconv.ParseFloat(tickers[0].Last, 64)
	if err != nil {
		return 0, err
	}

	return price, nil
}

// SetLeverage 设置杠杆
func (t *GateIOFuturesTrader) SetLeverage(symbol string, leverage int) error {
	// 转换符号格式
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	// 验证杠杆值
	if leverage <= 0 {
		return fmt.Errorf("杠杆值必须大于0: %d", leverage)
	}

	// 使用 SDK 设置杠杆
	_, resp, err := t.client.FuturesApi.UpdatePositionLeverage(t.ctx, "usdt", gateIOSymbol, strconv.Itoa(leverage), nil)
	if err != nil {
		// Gate.io API 在某些情况下（如没有持仓时）可能返回数组而不是单个对象
		// 如果错误是 JSON 解析错误但 HTTP 状态码是成功的，可以认为设置成功
		if resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if strings.Contains(err.Error(), "cannot unmarshal array") || strings.Contains(err.Error(), "unmarshal") {
				log.Printf("  ✓ %s 杠杆已设置为 %dx (API返回数组格式，但设置成功)", symbol, leverage)
				return nil
			}
		}
		
		// 如果错误信息包含"already"，说明杠杆已经是目标值
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "same") {
			log.Printf("  ✓ %s 杠杆已是 %dx", symbol, leverage)
			return nil
		}
		log.Printf("❌ [SetLeverage] %s 设置杠杆失败 - 错误详情: %v", symbol, err)
		return fmt.Errorf("设置杠杆失败: %w", err)
	}

	log.Printf("  ✓ %s 杠杆已切换为 %dx", symbol, leverage)
	return nil
}

// SetMarginMode 设置仓位模式 (true=全仓, false=逐仓)
func (t *GateIOFuturesTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// 转换符号格式
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	change := "isolated"
	if isCrossMargin {
		change = "cross"
	}

	// 使用 SDK 设置仓位模式
	_, _, err := t.client.FuturesApi.UpdatePositionMargin(t.ctx, "usdt", gateIOSymbol, change)
	if err != nil {
		// 如果错误信息包含"already"或"same"，说明已经是目标模式
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "same") {
			modeStr := "全仓"
			if !isCrossMargin {
				modeStr = "逐仓"
			}
			log.Printf("  ✓ %s 仓位模式已是 %s", symbol, modeStr)
			return nil
		}
		// 如果有持仓，可能无法更改仓位模式
		if strings.Contains(err.Error(), "position") {
			log.Printf("  ⚠️ %s 有持仓，无法更改仓位模式，继续使用当前模式", symbol)
			return nil
		}
		log.Printf("  ⚠️ 设置仓位模式失败: %v", err)
		return nil // 不返回错误，让交易继续
	}

	modeStr := "全仓"
	if !isCrossMargin {
		modeStr = "逐仓"
	}
	log.Printf("  ✓ %s 仓位模式已设置为 %s", symbol, modeStr)
	return nil
}

// OpenLong 开多仓
func (t *GateIOFuturesTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 先取消该币种的所有委托单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消旧委托单失败（可能没有委托单）: %v", err)
	}

	// 设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// 格式化数量（内部会检查最小订单数量和精度要求）
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// 解析格式化后的数量（合约数量）用于后续检查
	contractSizeFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil {
		return nil, fmt.Errorf("解析格式化后的数量失败: %w", parseErr)
	}

	// 检查最小名义价值（使用币种数量）
	if err := t.CheckMinNotional(symbol, quantity); err != nil {
		return nil, err
	}

	// 转换符号格式
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	// 将合约数量转换为 int64（正数表示买入/开多）
	// 注意：Gate.io 的 Size 字段是 int64，表示合约数量
	quantityInt64 := int64(contractSizeFloat)

	// 使用 SDK 创建订单（市价单，正数size表示买入/开多）
	// 注意：对于市价单，Price 需要设置为 "0"
	// 注意：CreateFuturesOrder 返回 gateapi.FuturesOrder 而不是 *gateapi.FuturesOrder
	order, _, err := t.client.FuturesApi.CreateFuturesOrder(t.ctx, "usdt", gateapi.FuturesOrder{
		Contract:   gateIOSymbol,
		Size:       quantityInt64, // 正数表示买入（开多）
		Price:      "0",            // 市价单设置为 "0"
		ReduceOnly: false,          // 开仓时设置为 false
		Tif:        "ioc",          // Immediate or Cancel
		Text:       "t-gateio-futures",
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("开多仓失败: %w", err)
	}

	log.Printf("✓ 开多仓成功: %s 数量: %s", symbol, quantityStr)
	log.Printf("  订单ID: %d", order.Id)

	result := make(map[string]interface{})
	result["orderId"] = order.Id
	result["symbol"] = order.Contract
	result["status"] = order.Status
	return result, nil
}

// OpenShort 开空仓
func (t *GateIOFuturesTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 先取消该币种的所有委托单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消旧委托单失败（可能没有委托单）: %v", err)
	}

	// 设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// 格式化数量（内部会检查最小订单数量和精度要求）
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// 解析格式化后的数量（合约数量）用于后续检查
	contractSizeFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil {
		return nil, fmt.Errorf("解析格式化后的数量失败: %w", parseErr)
	}

	// 检查最小名义价值（使用币种数量）
	if err := t.CheckMinNotional(symbol, quantity); err != nil {
		return nil, err
	}

	// 转换符号格式
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	// 将合约数量转为负数（int64格式，负数表示卖出/开空）
	negQuantityInt64 := int64(-contractSizeFloat)

	// 使用 SDK 创建订单（市价单，负数size表示卖出/开空）
	// 注意：对于市价单，Price 需要设置为 "0"
	// 注意：CreateFuturesOrder 返回 gateapi.FuturesOrder 而不是 *gateapi.FuturesOrder
	order, _, err := t.client.FuturesApi.CreateFuturesOrder(t.ctx, "usdt", gateapi.FuturesOrder{
		Contract:   gateIOSymbol,
		Size:       negQuantityInt64, // 负数表示开空
		Price:      "0",              // 市价单设置为 "0"
		ReduceOnly: false,          // 开仓时设置为 false
		Tif:        "ioc",            // Immediate or Cancel
		Text:       "t-gateio-futures",
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("开空仓失败: %w", err)
	}

	log.Printf("✓ 开空仓成功: %s 数量: %s", symbol, quantityStr)
	log.Printf("  订单ID: %d", order.Id)

	result := make(map[string]interface{})
	result["orderId"] = order.Id
	result["symbol"] = order.Contract
	result["status"] = order.Status
	return result, nil
}

// CloseLong 平多仓
func (t *GateIOFuturesTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	// 记录是否要平掉所有持仓
	closeAll := (quantity == 0)
	
	// 如果数量为0，获取当前持仓数量（币种数量）
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				// 使用安全的类型断言
				if amt, ok := pos["positionAmt"].(float64); ok {
					quantity = amt
				} else {
					return nil, fmt.Errorf("无法获取 %s 的多仓数量（类型错误）", symbol)
				}
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的多仓", symbol)
		}
	}

	// 转换符号格式
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	// 格式化数量（内部会检查最小订单数量和精度要求）
	// 注意：即使 closeAll，也需要将币种数量转换为合约数量
	quantityStr, formatErr := t.FormatQuantity(symbol, quantity)
	if formatErr != nil {
		return nil, formatErr
	}

	// 解析格式化后的数量（合约数量）
	contractSizeFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil {
		return nil, fmt.Errorf("解析格式化后的数量失败: %w", parseErr)
	}

	// 将合约数量转换为 int64（负数表示卖出/平多仓）
	contractSizeInt64 := int64(contractSizeFloat)
	if contractSizeInt64 < 0 {
		contractSizeInt64 = -contractSizeInt64 // 确保为正数，然后转为负数
	}
	
	// 根据 Gate.io 文档：
	// - 在双仓模式下，不能使用 Close: true（会报错 "close is not allowed in dual-mode"）
	// - 必须使用 reduce_only: true 并指定具体的 Size
	// - 平多仓：Size 为负数（卖出）
	// 注意：即使要平掉所有持仓，也使用 reduce_only: true，因为账户可能是双仓模式
	var order gateapi.FuturesOrder
	order, _, err := t.client.FuturesApi.CreateFuturesOrder(t.ctx, "usdt", gateapi.FuturesOrder{
		Contract:   gateIOSymbol,
		Size:       -contractSizeInt64, // 负数表示卖出（平多仓）
		Price:      "0",                 // 市价单设置为 "0"
		ReduceOnly: true,               // 防止减仓时被穿透仓位，双仓模式下必须使用此方式
		Tif:        "ioc",               // Immediate or Cancel
		Text:       "t-gateio-futures-close",
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("平多仓失败: %w", err)
	}

	if closeAll {
		log.Printf("✓ 平多仓成功: %s (全部平仓)", symbol)
	} else {
		log.Printf("✓ 平多仓成功: %s 数量: %.8f (币种数量)", symbol, quantity)
	}

	// 平仓后取消该币种的所有挂单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = order.Id
	result["symbol"] = order.Contract
	result["status"] = order.Status
	return result, nil
}

// CloseShort 平空仓
func (t *GateIOFuturesTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	// 记录是否要平掉所有持仓
	closeAll := (quantity == 0)
	
	// 如果数量为0，获取当前持仓数量（币种数量）
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				// 使用安全的类型断言
				if amt, ok := pos["positionAmt"].(float64); ok {
					quantity = amt
				} else {
					return nil, fmt.Errorf("无法获取 %s 的空仓数量（类型错误）", symbol)
				}
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的空仓", symbol)
		}
	}

	// 转换符号格式
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	// 格式化数量（内部会检查最小订单数量和精度要求）
	// 注意：即使 closeAll，也需要将币种数量转换为合约数量
	quantityStr, formatErr := t.FormatQuantity(symbol, quantity)
	if formatErr != nil {
		return nil, formatErr
	}

	// 解析格式化后的数量（合约数量）
	contractSizeFloat, parseErr := strconv.ParseFloat(quantityStr, 64)
	if parseErr != nil {
		return nil, fmt.Errorf("解析格式化后的数量失败: %w", parseErr)
	}

	// 将合约数量转换为 int64（正数表示买入/平空仓）
	contractSizeInt64 := int64(contractSizeFloat)
	if contractSizeInt64 < 0 {
		contractSizeInt64 = -contractSizeInt64 // 确保为正数
	}
	
	// 根据 Gate.io 文档：
	// - 在双仓模式下，不能使用 Close: true（会报错 "close is not allowed in dual-mode"）
	// - 必须使用 reduce_only: true 并指定具体的 Size
	// - 平空仓：Size 为正数（买入）
	// 注意：即使要平掉所有持仓，也使用 reduce_only: true，因为账户可能是双仓模式
	var order gateapi.FuturesOrder
	order, _, err := t.client.FuturesApi.CreateFuturesOrder(t.ctx, "usdt", gateapi.FuturesOrder{
		Contract:   gateIOSymbol,
		Size:       contractSizeInt64, // 正数表示买入（平空仓）
		Price:      "0",               // 市价单设置为 "0"
		ReduceOnly: true,             // 防止减仓时被穿透仓位，双仓模式下必须使用此方式
		Tif:        "ioc",             // Immediate or Cancel
		Text:       "t-gateio-futures-close",
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("平空仓失败: %w", err)
	}

	if closeAll {
		log.Printf("✓ 平空仓成功: %s (全部平仓)", symbol)
	} else {
		log.Printf("✓ 平空仓成功: %s 数量: %.8f (币种数量)", symbol, quantity)
	}

	// 平仓后取消该币种的所有挂单
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	result := make(map[string]interface{})
	result["orderId"] = order.Id
	result["symbol"] = order.Contract
	result["status"] = order.Status
	return result, nil
}

// CancelAllOrders 取消该币种的所有挂单（包括普通订单和价格触发订单）
func (t *GateIOFuturesTrader) CancelAllOrders(symbol string) error {
	// 转换符号格式
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	// 取消普通订单
	_, _, err := t.client.FuturesApi.CancelFuturesOrders(t.ctx, "usdt", gateIOSymbol, nil)
	if err != nil {
		// 如果没有订单，可能返回错误，但不影响
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "no order") {
			log.Printf("  ⚠ 取消普通订单失败: %v", err)
		}
	}

	// 取消价格触发订单（止损/止盈单）
	// 使用 ListPriceTriggeredOrders 获取该币种的所有价格触发订单
	// 注意：ListPriceTriggeredOrders 需要4个参数：ctx, settle, status, opts
	opts := &gateapi.ListPriceTriggeredOrdersOpts{}
	if gateIOSymbol != "" {
		opts.Contract = optional.NewString(gateIOSymbol)
	}
	// status: "open" 表示未触发的订单，"finish" 表示已触发的订单，空字符串表示所有
	priceOrders, _, err := t.client.FuturesApi.ListPriceTriggeredOrders(t.ctx, "usdt", "open", opts)
	if err == nil && len(priceOrders) > 0 {
		// 逐个取消价格触发订单
		for _, order := range priceOrders {
			// order.Id 是 int64 类型，需要转换为字符串
			if order.Id > 0 {
				orderIdStr := strconv.FormatInt(order.Id, 10)
				_, _, cancelErr := t.client.FuturesApi.CancelPriceTriggeredOrder(t.ctx, "usdt", orderIdStr)
				if cancelErr != nil {
					log.Printf("  ⚠ 取消价格触发订单 %d 失败: %v", order.Id, cancelErr)
				}
			}
		}
		log.Printf("  ✓ 已取消 %s 的 %d 个价格触发订单", symbol, len(priceOrders))
	}

	log.Printf("  ✓ 已取消 %s 的所有挂单", symbol)
	return nil
}

// CancelStopLossOrders 仅取消止损单
func (t *GateIOFuturesTrader) CancelStopLossOrders(symbol string) error {
	// 转换符号格式
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	// 获取当前价格，用于判断止损/止盈
	currentPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		log.Printf("  ⚠ 获取 %s 当前价格失败，将取消所有价格触发订单: %v", symbol, err)
		currentPrice = 0 // 如果无法获取价格，则取消所有价格触发订单
	}

	// 使用 SDK 获取价格触发订单列表（止损/止盈单）
	// 注意：ListPriceTriggeredOrders 需要4个参数：ctx, settle, status, opts
	opts := &gateapi.ListPriceTriggeredOrdersOpts{}
	if gateIOSymbol != "" {
		opts.Contract = optional.NewString(gateIOSymbol)
	}
	// status: "open" 表示未触发的订单
	priceOrders, _, err := t.client.FuturesApi.ListPriceTriggeredOrders(t.ctx, "usdt", "open", opts)
	if err != nil {
		return fmt.Errorf("获取价格触发订单失败: %w", err)
	}

	// 获取持仓信息，判断持仓方向
	positions, err := t.GetPositions()
	if err != nil {
		log.Printf("  ⚠ 获取持仓信息失败: %v", err)
	}

	var positionSide string
	for _, pos := range positions {
		if pos["symbol"] == symbol {
			positionSide = pos["side"].(string)
			break
		}
	}

	canceledCount := 0
	for _, order := range priceOrders {
		// 判断是否为止损单
		// 止损单的判断逻辑：
		// - 多仓（LONG）：触发价格 < 当前价格（价格下跌触发止损）
		// - 空仓（SHORT）：触发价格 > 当前价格（价格上涨触发止损）
		isStopLoss := false
		
		// order.Trigger 不是指针类型，直接检查 Price 字段
		if order.Trigger.Price != "" {
			triggerPrice, parseErr := strconv.ParseFloat(order.Trigger.Price, 64)
			if parseErr == nil && currentPrice > 0 {
				if positionSide == "long" {
					// 多仓：触发价格低于当前价格为止损
					isStopLoss = triggerPrice < currentPrice
				} else if positionSide == "short" {
					// 空仓：触发价格高于当前价格为止损
					isStopLoss = triggerPrice > currentPrice
				}
			} else {
				// 如果无法判断，根据订单的size方向判断
				// 止损单通常是平仓订单，size应该与持仓方向相反
				// order.Initial 不是指针类型，直接访问
				size := order.Initial.Size
				if positionSide == "long" && size < 0 {
					isStopLoss = true // 多仓止损，size为负（卖出）
				} else if positionSide == "short" && size > 0 {
					isStopLoss = true // 空仓止损，size为正（买入）
				}
			}
		}

		// 如果无法判断持仓方向或价格，跳过该订单（避免误取消）
		if positionSide == "" || currentPrice == 0 {
			log.Printf("  ⚠ 无法判断 %s 的止损单（缺少持仓或价格信息），跳过订单 %d", symbol, order.Id)
			continue
		}

		if isStopLoss && order.Id > 0 {
			// order.Id 是 int64 类型，需要转换为字符串
			orderIdStr := strconv.FormatInt(order.Id, 10)
			_, _, cancelErr := t.client.FuturesApi.CancelPriceTriggeredOrder(t.ctx, "usdt", orderIdStr)
			if cancelErr != nil {
				log.Printf("  ⚠ 取消止损单 %d 失败: %v", order.Id, cancelErr)
				continue
			}
			canceledCount++
			log.Printf("  ✓ 已取消止损单 (订单ID: %d)", order.Id)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s 没有止损单需要取消", symbol)
	} else {
		log.Printf("  ✓ 已取消 %s 的 %d 个止损单", symbol, canceledCount)
	}

	return nil
}

// CancelTakeProfitOrders 仅取消止盈单
func (t *GateIOFuturesTrader) CancelTakeProfitOrders(symbol string) error {
	// 转换符号格式
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	// 获取当前价格，用于判断止损/止盈
	currentPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		log.Printf("  ⚠ 获取 %s 当前价格失败，将取消所有价格触发订单: %v", symbol, err)
		currentPrice = 0 // 如果无法获取价格，则取消所有价格触发订单
	}

	// 使用 SDK 获取价格触发订单列表（止损/止盈单）
	// 注意：ListPriceTriggeredOrders 需要4个参数：ctx, settle, status, opts
	opts := &gateapi.ListPriceTriggeredOrdersOpts{}
	if gateIOSymbol != "" {
		opts.Contract = optional.NewString(gateIOSymbol)
	}
	// status: "open" 表示未触发的订单
	priceOrders, _, err := t.client.FuturesApi.ListPriceTriggeredOrders(t.ctx, "usdt", "open", opts)
	if err != nil {
		return fmt.Errorf("获取价格触发订单失败: %w", err)
	}

	// 获取持仓信息，判断持仓方向
	positions, err := t.GetPositions()
	if err != nil {
		log.Printf("  ⚠ 获取持仓信息失败: %v", err)
	}

	var positionSide string
	for _, pos := range positions {
		if pos["symbol"] == symbol {
			positionSide = pos["side"].(string)
			break
		}
	}

	canceledCount := 0
	for _, order := range priceOrders {
		// 判断是否为止盈单
		// 止盈单的判断逻辑：
		// - 多仓（LONG）：触发价格 > 当前价格（价格上涨触发止盈）
		// - 空仓（SHORT）：触发价格 < 当前价格（价格下跌触发止盈）
		isTakeProfit := false
		
		// order.Trigger 不是指针类型，直接检查 Price 字段
		if order.Trigger.Price != "" {
			triggerPrice, parseErr := strconv.ParseFloat(order.Trigger.Price, 64)
			if parseErr == nil && currentPrice > 0 {
				if positionSide == "long" {
					// 多仓：触发价格高于当前价格为止盈
					isTakeProfit = triggerPrice > currentPrice
				} else if positionSide == "short" {
					// 空仓：触发价格低于当前价格为止盈
					isTakeProfit = triggerPrice < currentPrice
				}
			} else {
				// 如果无法判断，根据订单的size方向判断
				// 止盈单通常是平仓订单，size应该与持仓方向相反
				// order.Initial 不是指针类型，直接访问
				size := order.Initial.Size
				if positionSide == "long" && size < 0 {
					isTakeProfit = true // 多仓止盈，size为负（卖出）
				} else if positionSide == "short" && size > 0 {
					isTakeProfit = true // 空仓止盈，size为正（买入）
				}
			}
		}

		// 如果无法判断持仓方向或价格，跳过该订单（避免误取消）
		if positionSide == "" || currentPrice == 0 {
			log.Printf("  ⚠ 无法判断 %s 的止盈单（缺少持仓或价格信息），跳过订单 %d", symbol, order.Id)
			continue
		}

		if isTakeProfit && order.Id > 0 {
			// order.Id 是 int64 类型，需要转换为字符串
			orderIdStr := strconv.FormatInt(order.Id, 10)
			_, _, cancelErr := t.client.FuturesApi.CancelPriceTriggeredOrder(t.ctx, "usdt", orderIdStr)
			if cancelErr != nil {
				log.Printf("  ⚠ 取消止盈单 %d 失败: %v", order.Id, cancelErr)
				continue
			}
			canceledCount++
			log.Printf("  ✓ 已取消止盈单 (订单ID: %d)", order.Id)
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s 没有止盈单需要取消", symbol)
	} else {
		log.Printf("  ✓ 已取消 %s 的 %d 个止盈单", symbol, canceledCount)
	}

	return nil
}

// CancelStopOrders 取消该币种的止盈/止损单
func (t *GateIOFuturesTrader) CancelStopOrders(symbol string) error {
	// 取消止损和止盈单
	if err := t.CancelStopLossOrders(symbol); err != nil {
		return err
	}
	if err := t.CancelTakeProfitOrders(symbol); err != nil {
		return err
	}
	return nil
}

// SetStopLoss 设置止损单
func (t *GateIOFuturesTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// 根据持仓方向确定size的正负
	// LONG持仓：平多需要卖出（负数size）
	// SHORT持仓：平空需要买入（正数size）
	size, _ := strconv.ParseFloat(quantityStr, 64)
	if positionSide == "LONG" {
		size = -size // 平多仓，size为负
	}
	sizeInt64 := int64(size)

	// 转换符号格式
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	// 使用 SDK 创建止损单
	_, _, err = t.client.FuturesApi.CreatePriceTriggeredOrder(t.ctx, "usdt", gateapi.FuturesPriceTriggeredOrder{
		Initial: gateapi.FuturesInitialOrder{
			Contract: gateIOSymbol,
			Size:     sizeInt64,
			Price:    fmt.Sprintf("%.8f", stopPrice), // 执行价格
			Tif:      "gtc",                          // Good Till Cancel
		},
		Trigger: gateapi.FuturesPriceTrigger{
			StrategyType: 0, // 0 = 价格触发
			PriceType:    0, // 0 = 最新价格
			Price:        fmt.Sprintf("%.8f", stopPrice), // 触发价格
		},
	})
	if err != nil {
		return fmt.Errorf("设置止损失败: %w", err)
	}

	log.Printf("  止损价设置: %.4f", stopPrice)
	return nil
}

// SetTakeProfit 设置止盈单
func (t *GateIOFuturesTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	quantityStr, err := t.FormatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// 根据持仓方向确定size的正负
	size, _ := strconv.ParseFloat(quantityStr, 64)
	if positionSide == "LONG" {
		size = -size // 平多仓，size为负
	}
	sizeInt64 := int64(size)

	// 转换符号格式
	gateIOSymbol := normalizeSymbolForGateIO(symbol)

	// 使用 SDK 创建止盈单
	_, _, err = t.client.FuturesApi.CreatePriceTriggeredOrder(t.ctx, "usdt", gateapi.FuturesPriceTriggeredOrder{
		Initial: gateapi.FuturesInitialOrder{
			Contract: gateIOSymbol,
			Size:     sizeInt64,
			Price:    fmt.Sprintf("%.8f", takeProfitPrice), // 执行价格
			Tif:      "gtc",
		},
		Trigger: gateapi.FuturesPriceTrigger{
			StrategyType: 0, // 0 = 价格触发
			PriceType:    0, // 0 = 最新价格
			Price:        fmt.Sprintf("%.8f", takeProfitPrice), // 触发价格
		},
	})
	if err != nil {
		return fmt.Errorf("设置止盈失败: %w", err)
	}

	log.Printf("  止盈价设置: %.4f", takeProfitPrice)
	return nil
}

// GetMinNotional 获取最小名义价值（Gate.io要求）
func (t *GateIOFuturesTrader) GetMinNotional(symbol string) float64 {
	// Gate.io的最小订单价值，使用保守的默认值
	return 10.0
}

// CheckMinNotional 检查订单是否满足最小名义价值要求
// quantity: 币种数量（如 BTC 的数量）
func (t *GateIOFuturesTrader) CheckMinNotional(symbol string, quantity float64) error {
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return fmt.Errorf("获取市价失败: %w", err)
	}

	// 名义价值 = 币种数量 × 价格
	notionalValue := quantity * price
	minNotional := t.GetMinNotional(symbol)

	if notionalValue < minNotional {
		return fmt.Errorf(
			"订单金额 %.2f USDT 低于最小要求 %.2f USDT (币种数量: %.8f, 价格: %.4f)",
			notionalValue, minNotional, quantity, price,
		)
	}

	return nil
}

// GetMinOpenAmount 获取币种的最小开仓金额（USDT）
// 考虑最小合约数量、精度、quanto_multiplier 等因素
func (t *GateIOFuturesTrader) GetMinOpenAmount(symbol string) (float64, error) {
	// 获取当前价格
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return 0, fmt.Errorf("获取市场价格失败: %w", err)
	}

	// 获取合约信息
	info, err := t.getSymbolInfo(symbol)
	if err != nil {
		// 如果无法获取合约信息，使用保守的默认值
		log.Printf("  ⚠ %s 未找到合约信息，使用默认最小开仓金额 12 USDT", symbol)
		return 12.0, nil
	}

	// 获取 quanto_multiplier
	quantoMultiplier := 1.0
	if info.QuantoMultiplier != "" {
		parsed, parseErr := strconv.ParseFloat(info.QuantoMultiplier, 64)
		if parseErr == nil && parsed > 0 {
			quantoMultiplier = parsed
		}
	}

	// 计算最小开仓金额（考虑最小合约数量和精度）
	var minNotional float64

	// 1. 检查最小合约数量（OrderSizeMin）
	if info.OrderSizeMin > 0 {
		minContractSize := float64(info.OrderSizeMin)
		minCoinQuantity := minContractSize * quantoMultiplier
		minNotional = minCoinQuantity * price
	} else {
		// 如果没有 OrderSizeMin，使用精度来计算
		precision := 3 // 默认精度
		if info.OrderPriceRound != "" {
			if strings.Contains(info.OrderPriceRound, ".") {
				parts := strings.Split(info.OrderPriceRound, ".")
				if len(parts) == 2 {
					precision = len(parts[1])
				}
			}
		}
		// 最小合约数量 = 1 / 10^precision
		minContractQuantity := 1.0 / math.Pow10(precision)
		minCoinQuantity := minContractQuantity * quantoMultiplier
		minNotional = minCoinQuantity * price
	}

	// 2. 确保不低于交易所的最小名义价值要求（10 USDT）
	minExchangeNotional := t.GetMinNotional(symbol)
	if minNotional < minExchangeNotional {
		minNotional = minExchangeNotional
	}

	// 3. 添加安全边际（10%）
	minNotional = minNotional * 1.1

	return minNotional, nil
}

