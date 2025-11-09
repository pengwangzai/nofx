# Gate.io 合约数量与币种数量转换说明

## 核心概念

### 1. 合约数量 (Contract Size)
- Gate.io API 中的 `Size` 字段表示的是**合约数量**（int64）
- 例如：`Size: 10` 表示 10 张合约

### 2. 币种数量 (Coin Quantity)
- 用户输入和系统内部使用的通常是**币种数量**（如 BTC 的数量）
- 例如：`0.001 BTC` 表示 0.001 个 BTC

### 3. quanto_multiplier（合约乘数）
- 每张合约对应的币种数量
- 例如：如果 `quanto_multiplier = 0.0001`，那么 1 张合约 = 0.0001 BTC
- 公式：`币种数量 = 合约数量 × quanto_multiplier`
- 反推：`合约数量 = 币种数量 ÷ quanto_multiplier`

## 转换逻辑

### 1. GetPositions() - 获取持仓
**输入**：API 返回的 `pos.Size`（合约数量，int64）

**处理**：
```go
contractSize := float64(pos.Size)  // 合约数量
coinQuantity := contractSize * quantoMultiplier  // 转换为币种数量
```

**输出**：`posMap["positionAmt"]` = 币种数量（float64）

**说明**：系统统一使用币种数量，方便用户理解和使用。

### 2. convertCoinQuantityToContractSize() - 币种数量 → 合约数量
**输入**：币种数量（float64）

**处理**：
```go
quantoMultiplier := 解析(info.QuantoMultiplier)  // 从合约信息获取
contractSize := coinQuantity / quantoMultiplier   // 转换为合约数量
```

**输出**：合约数量（float64）

**说明**：用于将用户输入的币种数量转换为 API 需要的合约数量。

### 3. FormatQuantity() - 格式化数量
**输入**：币种数量（float64）

**处理流程**：
1. 调用 `convertCoinQuantityToContractSize()` 转换为合约数量
2. 检查最小订单数量（`OrderSizeMin`，基于合约数量）
3. 应用精度格式化（根据 `OrderPriceRound` 推断）
4. 检查格式化后是否为 0

**输出**：格式化后的合约数量字符串

**说明**：确保合约数量满足最小订单要求和精度要求。

### 4. OpenLong/OpenShort() - 开仓
**输入**：币种数量（float64）

**处理流程**：
1. 调用 `FormatQuantity()` 得到格式化后的合约数量字符串
2. 解析为 `contractSizeFloat`（合约数量）
3. 转换为 `int64` 作为 API 的 `Size` 参数
4. 创建订单

**API 调用**：
```go
CreateFuturesOrder(..., gateapi.FuturesOrder{
    Size: int64(contractSizeFloat),  // 合约数量（正数=买入，负数=卖出）
    ...
})
```

### 5. CloseLong/CloseShort() - 平仓
**输入**：币种数量（float64），0 表示平掉所有持仓

**处理流程**：

#### 情况 A：平掉所有持仓（quantity == 0）
1. 从持仓获取币种数量
2. 直接使用 `Close: true, Size: 0` 创建订单

```go
CreateFuturesOrder(..., gateapi.FuturesOrder{
    Size: 0,
    Close: true,  // 平掉所有持仓
    ...
})
```

#### 情况 B：平掉指定数量（quantity > 0）
1. 调用 `FormatQuantity()` 将币种数量转换为合约数量
2. 根据方向设置正负号：
   - 平多仓：`Size` 为负数（卖出）
   - 平空仓：`Size` 为正数（买入）
3. 使用 `reduce_only: true` 创建订单

```go
// 平多仓
CreateFuturesOrder(..., gateapi.FuturesOrder{
    Size: -contractSizeInt64,  // 负数表示卖出
    ReduceOnly: true,
    ...
})

// 平空仓
CreateFuturesOrder(..., gateapi.FuturesOrder{
    Size: contractSizeInt64,  // 正数表示买入
    ReduceOnly: true,
    ...
})
```

### 6. SetStopLoss/SetTakeProfit() - 设置止损/止盈
**输入**：币种数量（float64）

**处理流程**：
1. 调用 `FormatQuantity()` 转换为合约数量
2. 根据持仓方向设置正负号：
   - LONG 持仓：`Size` 为负数（平多仓需要卖出）
   - SHORT 持仓：`Size` 为正数（平空仓需要买入）

**API 调用**：
```go
CreatePriceTriggeredOrder(..., gateapi.FuturesPriceTriggeredOrder{
    Initial: gateapi.FuturesInitialOrder{
        Size: int64(contractSize),  // 合约数量（带正负号）
        ...
    },
    ...
})
```

## 数量转换流程图

```
用户输入（币种数量）
    ↓
FormatQuantity()
    ↓
convertCoinQuantityToContractSize()
    ↓
币种数量 ÷ quanto_multiplier = 合约数量
    ↓
格式化（精度、最小订单检查）
    ↓
API 调用（使用合约数量）
```

## 关键修复点

### 1. 类型安全
- ✅ 使用安全的类型断言 `pos["positionAmt"].(float64)` 改为 `if amt, ok := pos["positionAmt"].(float64); ok`
- ✅ 避免类型不匹配导致的 panic

### 2. 平仓逻辑优化
- ✅ 使用 `closeAll` 标志记录是否要平掉所有持仓
- ✅ 平掉所有持仓时直接使用 `Close: true, Size: 0`，不进行数量转换
- ✅ 平掉指定数量时才进行币种数量到合约数量的转换

### 3. 数量转换一致性
- ✅ 所有输入统一为币种数量
- ✅ 所有 API 调用统一使用合约数量
- ✅ 所有输出统一为币种数量

## 示例

### 示例 1：BTC 合约
假设：
- `quanto_multiplier = 0.0001`（1 张合约 = 0.0001 BTC）
- 用户想开 0.001 BTC 的多仓

**转换过程**：
1. 输入：`0.001` BTC（币种数量）
2. 转换：`0.001 ÷ 0.0001 = 10` 张合约
3. API 调用：`Size: 10`（合约数量）

### 示例 2：平仓
假设：
- 当前持仓：`0.001` BTC（币种数量）
- 用户想平掉 `0.0005` BTC

**转换过程**：
1. 输入：`0.0005` BTC（币种数量）
2. 转换：`0.0005 ÷ 0.0001 = 5` 张合约
3. API 调用：`Size: -5`（负数表示卖出，平多仓）

### 示例 3：平掉所有持仓
假设：
- 当前持仓：`0.001` BTC（币种数量）
- 用户输入：`0`（表示平掉所有）

**处理**：
- 直接使用 `Close: true, Size: 0`，不进行数量转换
- API 会自动平掉所有持仓

## 注意事项

1. **quanto_multiplier 可能为 0 或空**
   - 默认使用 `1.0`（1:1 合约）
   - 记录警告日志

2. **精度问题**
   - 合约数量需要满足最小订单要求（`OrderSizeMin`）
   - 需要满足精度要求（根据 `OrderPriceRound` 推断）

3. **数量方向**
   - 开多仓：`Size` 为正数
   - 开空仓：`Size` 为负数
   - 平多仓：`Size` 为负数（卖出）
   - 平空仓：`Size` 为正数（买入）

4. **空仓数量处理**
   - 在 `GetPositions()` 中，空仓的 `positionAmt` 存储为正数
   - 在 `CloseShort()` 中，直接使用该正数进行转换

