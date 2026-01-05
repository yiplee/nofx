# 回测执行流程详解

## 完整流程概览

```
┌─────────────────────────────────────────────────────────────────┐
│                    回测执行完整流程                               │
└─────────────────────────────────────────────────────────────────┘

1. API 请求: POST /api/backtest/start
   ↓
2. handleBacktestStart() (api/backtest.go:55)
   ├─ 解析请求参数 (BacktestConfig)
   ├─ 加载策略配置 (如果提供了 strategy_id)
   ├─ 解析 AI 模型配置 (hydrateBacktestAIConfig)
   └─ 调用 backtestManager.Start()
   ↓
3. Manager.Start() (backtest/manager.go:43)
   ├─ 验证配置 (cfg.Validate())
   ├─ 解析 AI 配置 (resolveAIConfig)
   ├─ 创建 DataFeed (NewDataFeed)
   │   └─ loadAll(): 加载所有历史 K 线数据
   │       ├─ 遍历所有 symbols 和 timeframes
   │       ├─ 调用 market.GetKlinesRange() 获取数据
   │       └─ 构建决策时间线 (decisionTimes)
   ├─ 创建 Runner 实例 (NewRunner)
   │   ├─ 配置 MCP Client (configureMCPClient)
   │   ├─ 创建 BacktestAccount
   │   ├─ 初始化状态 (BacktestState)
   │   └─ 创建 StrategyEngine
   └─ 启动 Runner.Start() (goroutine)
   ↓
4. Runner.Start() → Runner.loop() (backtest/runner.go:232)
   └─ 循环执行 stepOnce()，直到完成或失败
   ↓
5. Runner.stepOnce() (backtest/runner.go:266)
   ├─ 获取当前时间戳 (DecisionTimestamp)
   ├─ 构建市场数据快照 (BuildMarketData)
   │   └─ 为每个 symbol 和 timeframe 切片 K 线数据
   ├─ 检查决策触发条件 (shouldTriggerDecision)
   │   └─ 每 N 根 K 线触发一次决策
   ├─ [如果触发] buildDecisionContext()
   │   ├─ 构建账户信息
   │   ├─ 构建持仓信息
   │   ├─ 获取候选币种
   │   ├─ 合并 multiTF 数据到 MarketDataMap (修复 quant 兼容性)
   │   └─ 获取量化数据 (如果启用)
   ├─ [如果触发] invokeAIWithRetry()
   │   ├─ 调用 decision.GetFullDecisionWithStrategy()
   │   │   ├─ 构建 System Prompt
   │   │   ├─ 构建 User Prompt
   │   │   └─ 调用 mcpClient.CallWithMessages()
   │   │       └─ QuantClient: 通过 meta["context"] 传递数据
   │   └─ 解析 AI 响应 (parseFullDecisionResponse)
   ├─ [如果触发] executeDecision()
   │   └─ 执行交易决策 (开仓/平仓)
   ├─ checkLiquidation()
   ├─ updateState()
   ├─ appendEquityPoint()
   ├─ appendTradeEvent()
   ├─ maybeCheckpoint()
   └─ persistMetrics()
   ↓
6. 完成/失败处理
   ├─ handleCompletion() / handleFailure()
   ├─ 计算最终指标
   ├─ 持久化所有结果
   └─ 释放锁
   ↓
7. API 查询结果
   ├─ GET /api/backtest/metrics
   ├─ GET /api/backtest/equity
   ├─ GET /api/backtest/trades
   └─ GET /api/backtest/decisions
```

## 关键数据结构

### BacktestConfig
```go
type BacktestConfig struct {
    RunID             string
    Symbols           []string
    Timeframes        []string      // 例如: ["1h"]
    DecisionTimeframe string         // 例如: "1h"
    StartTS           int64
    EndTS             int64
    InitialBalance    float64
    // ... 其他配置
}
```

### DataFeed
```go
type DataFeed struct {
    symbols       []string
    timeframes    []string
    symbolSeries  map[string]*symbolSeries  // symbol -> timeframe -> klines
    decisionTimes []int64                    // 决策时间点列表
    primaryTF     string                     // 主时间周期
}
```

### MarketDataMap (传递给 AI)
```go
// 修复前: 只包含 primary timeframe 的数据
MarketDataMap[symbol].TimeframeData[primaryTF] = data

// 修复后: 包含所有 timeframes 的数据
MarketDataMap[symbol].TimeframeData["1h"] = data
MarketDataMap[symbol].TimeframeData["4h"] = data  // 如果配置了多个
```

## Quant Client 数据传递流程

### 1. NOFX 端 (backtest/runner.go)
```go
// buildDecisionContext 中
ctx := &decision.Context{
    MarketDataMap: mergedMarketData,  // 包含所有 timeframes
    Timeframes:   r.cfg.Timeframes,   // ["1h"]
}

// GetFullDecisionWithStrategy 中
if withMeta, ok := mcpClient.(mcp.AIClientWithMeta); ok {
    mcpClient = withMeta.WithMeta("context", ctx)  // 通过 meta 传递
}
```

### 2. Quant Client (mcp/quant_client.go)
```go
// marshalRequestBody 中
body := map[string]any{
    "meta": quantClient.meta,  // 包含 context
}
```

### 3. Quant 服务端 (quant/handler/api/server.go)
```go
// 解析 context
if contextData, ok := req.Meta["context"]; ok {
    decisionCtx := new(DecisionContext)
    json.Unmarshal(contextData, decisionCtx)
}

// 使用数据
marketData := decisionCtx.MarketDataMap[symbol]
tfData := marketData.TimeframeData["1h"]  // 需要这个数据存在
```

## 问题修复说明

### 问题
创建 timeframes=[1h] 的回测时，quant 返回 "No kline data for timeframe 1h"

### 根本原因
- `BuildMarketData()` 返回的 `marketData` 只包含 primary timeframe 的数据
- `multiTF` 包含所有 timeframes 的数据，但没有合并到 `MarketDataMap` 中
- Quant client 期望 `MarketDataMap[symbol].TimeframeData[timeframe]` 包含所有 timeframes 的数据

### 修复方案
在 `buildDecisionContext()` 中，将 `multiTF` 的数据合并到 `MarketDataMap` 中：

```go
// 合并 multiTF 数据到 MarketDataMap
mergedMarketData := make(map[string]*market.Data, len(marketData))
for symbol, primaryData := range marketData {
    merged := *primaryData
    if merged.TimeframeData == nil {
        merged.TimeframeData = make(map[string]*market.TimeframeSeriesData)
    }
    
    // 从 multiTF 合并所有 timeframe 数据
    if symbolTFs, ok := multiTF[symbol]; ok {
        for _, tfData := range symbolTFs {
            if tfData != nil && tfData.TimeframeData != nil {
                for tfKey, tfSeriesData := range tfData.TimeframeData {
                    if tfSeriesData != nil {
                        merged.TimeframeData[tfKey] = tfSeriesData
                    }
                }
            }
        }
    }
    
    mergedMarketData[symbol] = &merged
}
```

### 修复效果
- ✅ Quant client 现在可以访问所有 timeframes 的数据
- ✅ `MarketDataMap[symbol].TimeframeData["1h"]` 现在包含正确的数据
- ✅ 回测可以正常使用 quant AI model

## 相关文件

- `api/backtest.go:55` - API 入口
- `backtest/manager.go:43` - Manager 启动逻辑
- `backtest/runner.go:266` - 主循环
- `backtest/runner.go:473` - 构建决策上下文 (修复位置)
- `backtest/datafeed.go:154` - 构建市场数据
- `decision/engine.go:207` - AI 决策调用
- `mcp/quant_client.go` - Quant client 实现
- `quant/handler/api/server.go` - Quant 服务端处理
