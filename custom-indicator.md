# 自定义技术指标开发指南

本文档详细说明如何在 NOFX 项目中添加新的技术指标，供策略使用。目前系统已支持 EMA、MACD、RSI、ATR 等指标。

---

## 目录

1. [架构概述](#架构概述)
2. [现有指标分析](#现有指标分析)
3. [后端添加自定义指标](#后端添加自定义指标)
4. [前端添加自定义指标](#前端添加自定义指标)
5. [完整示例：添加 Bollinger Bands (BB)](#完整示例添加-bollinger-bands-bb)
6. [测试和验证](#测试和验证)
7. [常见问题](#常见问题)

---

## 架构概述

NOFX 的技术指标系统采用前后端分离架构：

```
┌─────────────────────────────────────────────────────────┐
│                     前端 (Web)                          │
│  ┌──────────────────────────────────────────────────┐  │
│  │  IndicatorEditor.tsx                             │  │
│  │  - 指标配置 UI                                    │  │
│  │  - 开关和参数设置                                 │  │
│  └──────────────────────────────────────────────────┘  │
│                        ↓                                │
│  ┌──────────────────────────────────────────────────┐  │
│  │  types.ts (IndicatorConfig)                      │  │
│  │  - TypeScript 类型定义                           │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
                        ↓ HTTP API
┌─────────────────────────────────────────────────────────┐
│                     后端 (Go)                            │
│  ┌──────────────────────────────────────────────────┐  │
│  │  store/strategy.go                               │  │
│  │  - IndicatorConfig 结构体定义                    │  │
│  │  - 配置序列化/反序列化                           │  │
│  └──────────────────────────────────────────────────┘  │
│                        ↓                                │
│  ┌──────────────────────────────────────────────────┐  │
│  │  market/data.go                                  │  │
│  │  - 指标计算函数 (calculateEMA, calculateRSI...) │  │
│  │  - 数据获取和组装                                │  │
│  └──────────────────────────────────────────────────┘  │
│                        ↓                                │
│  ┌──────────────────────────────────────────────────┐  │
│  │  market/types.go                                 │  │
│  │  - Data 结构体 (存储指标值)                      │  │
│  │  - TimeframeSeriesData (时间序列数据)            │  │
│  └──────────────────────────────────────────────────┘  │
│                        ↓                                │
│  ┌──────────────────────────────────────────────────┐  │
│  │  decision/engine.go                              │  │
│  │  - formatMarketData() 格式化指标输出             │  │
│  │  - BuildUserPrompt() 构建 AI 提示词             │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

---

## 现有指标分析

### 已支持的指标

| 指标 | 后端计算函数 | 配置字段 | 参数配置 | 数据结构字段 |
|------|------------|---------|---------|-------------|
| **EMA** | `calculateEMA()` | `EnableEMA` | `EMAPeriods` (数组) | `CurrentEMA20`, `EMA20Values[]`, `EMA50Values[]` |
| **MACD** | `calculateMACD()` | `EnableMACD` | 固定 12/26/9 | `CurrentMACD`, `MACDValues[]` |
| **RSI** | `calculateRSI()` | `EnableRSI` | `RSIPeriods` (数组) | `CurrentRSI7`, `RSI7Values[]`, `RSI14Values[]` |
| **ATR** | `calculateATR()` | `EnableATR` | `ATRPeriods` (数组) | `ATR14` (单值) |

### 指标数据流向

```
1. 市场数据获取 (market/data.go)
   ↓
2. 指标计算 (calculateXXX 函数)
   ↓
3. 数据组装 (Data 结构体)
   ↓
4. 格式化输出 (decision/engine.go:formatMarketData)
   ↓
5. AI 提示词构建 (BuildUserPrompt)
   ↓
6. AI 分析决策
```

---

## 后端添加自定义指标

### 步骤 1: 定义指标计算函数

在 `market/data.go` 中添加指标计算函数：

```go
// calculateBollingerBands 计算布林带指标
// 返回: [upperBand, middleBand, lowerBand]
func calculateBollingerBands(klines []Kline, period int, stdDev float64) (float64, float64, float64) {
    if len(klines) < period {
        return 0, 0, 0
    }
    
    // 计算 SMA (middle band)
    sum := 0.0
    for i := len(klines) - period; i < len(klines); i++ {
        sum += klines[i].Close
    }
    middleBand := sum / float64(period)
    
    // 计算标准差
    variance := 0.0
    for i := len(klines) - period; i < len(klines); i++ {
        diff := klines[i].Close - middleBand
        variance += diff * diff
    }
    stdDeviation := math.Sqrt(variance / float64(period))
    
    // 计算上下轨
    upperBand := middleBand + (stdDeviation * stdDev)
    lowerBand := middleBand - (stdDeviation * stdDev)
    
    return upperBand, middleBand, lowerBand
}
```

### 步骤 2: 更新数据结构

在 `market/types.go` 中添加指标字段：

```go
// Data 结构体添加字段
type Data struct {
    // ... 现有字段 ...
    
    // Bollinger Bands
    CurrentBBUpper  float64  // 当前上轨
    CurrentBBMiddle float64  // 当前中轨
    CurrentBBLower  float64  // 当前下轨
}

// TimeframeSeriesData 结构体添加字段
type TimeframeSeriesData struct {
    // ... 现有字段 ...
    
    BBUpperValues  []float64 `json:"bb_upper_values"`  // 上轨序列
    BBMiddleValues []float64 `json:"bb_middle_values"` // 中轨序列
    BBLowerValues  []float64 `json:"bb_lower_values"` // 下轨序列
}
```

### 步骤 3: 在数据获取函数中计算指标

在 `market/data.go` 的 `GetWithTimeframes()` 函数中添加计算逻辑：

```go
func GetWithTimeframes(symbol string, timeframes []string, primaryTimeframe string, count int) (*Data, error) {
    // ... 现有代码 ...
    
    // 计算当前指标（基于主时间周期）
    currentPrice := primaryKlines[len(primaryKlines)-1].Close
    currentEMA20 := calculateEMA(primaryKlines, 20)
    currentMACD := calculateMACD(primaryKlines)
    currentRSI7 := calculateRSI(primaryKlines, 7)
    
    // 新增：计算 Bollinger Bands
    bbUpper, bbMiddle, bbLower := calculateBollingerBands(primaryKlines, 20, 2.0)
    
    return &Data{
        Symbol:        symbol,
        CurrentPrice:  currentPrice,
        CurrentEMA20:  currentEMA20,
        CurrentMACD:  currentMACD,
        CurrentRSI7:  currentRSI7,
        // 新增字段
        CurrentBBUpper:  bbUpper,
        CurrentBBMiddle: bbMiddle,
        CurrentBBLower:  bbLower,
        // ... 其他字段 ...
    }, nil
}
```

### 步骤 4: 在时间序列计算中添加指标

在 `calculateTimeframeSeries()` 函数中添加：

```go
func calculateTimeframeSeries(klines []Kline, timeframe string, count int) *TimeframeSeriesData {
    data := &TimeframeSeriesData{
        // ... 现有字段 ...
        BBUpperValues:  make([]float64, 0, count),
        BBMiddleValues: make([]float64, 0, count),
        BBLowerValues:  make([]float64, 0, count),
    }
    
    // ... 现有循环代码 ...
    
    for i := start; i < len(klines); i++ {
        // ... 现有指标计算 ...
        
        // 新增：计算 Bollinger Bands 序列
        if i >= 19 { // 需要至少 20 根 K 线
            bbUpper, bbMiddle, bbLower := calculateBollingerBands(klines[:i+1], 20, 2.0)
            data.BBUpperValues = append(data.BBUpperValues, bbUpper)
            data.BBMiddleValues = append(data.BBMiddleValues, bbMiddle)
            data.BBLowerValues = append(data.BBLowerValues, bbLower)
        }
    }
    
    return data
}
```

### 步骤 5: 更新配置结构体

在 `store/strategy.go` 的 `IndicatorConfig` 中添加配置字段：

```go
type IndicatorConfig struct {
    // ... 现有字段 ...
    
    // Bollinger Bands
    EnableBB         bool    `json:"enable_bb"`
    BBPeriod         int     `json:"bb_period,omitempty"`         // 默认 20
    BBStdDev         float64 `json:"bb_std_dev,omitempty"`        // 默认 2.0
}
```

### 步骤 6: 更新格式化输出函数

在 `decision/engine.go` 的 `formatMarketData()` 函数中添加格式化逻辑：

```go
func (e *StrategyEngine) formatMarketData(data *market.Data) string {
    var sb strings.Builder
    indicators := e.config.Indicators
    
    sb.WriteString(fmt.Sprintf("current_price = %.4f", data.CurrentPrice))
    
    // ... 现有指标格式化 ...
    
    // 新增：Bollinger Bands 格式化
    if indicators.EnableBB {
        sb.WriteString(fmt.Sprintf(", current_bb_upper = %.3f, current_bb_middle = %.3f, current_bb_lower = %.3f",
            data.CurrentBBUpper, data.CurrentBBMiddle, data.CurrentBBLower))
    }
    
    sb.WriteString("\n\n")
    
    // ... 其他格式化代码 ...
}
```

在 `formatTimeframeSeriesData()` 函数中添加：

```go
func (e *StrategyEngine) formatTimeframeSeriesData(sb *strings.Builder, data *market.TimeframeSeriesData, indicators store.IndicatorConfig) {
    // ... 现有格式化代码 ...
    
    // 新增：Bollinger Bands 序列格式化
    if indicators.EnableBB {
        if len(data.BBUpperValues) > 0 {
            sb.WriteString(fmt.Sprintf("BB Upper: %s\n", formatFloatSlice(data.BBUpperValues)))
            sb.WriteString(fmt.Sprintf("BB Middle: %s\n", formatFloatSlice(data.BBMiddleValues)))
            sb.WriteString(fmt.Sprintf("BB Lower: %s\n", formatFloatSlice(data.BBLowerValues)))
        }
    }
    
    sb.WriteString("\n")
}
```

### 步骤 7: 更新默认配置

在 `store/strategy.go` 的 `GetDefaultStrategyConfig()` 函数中添加默认值：

```go
func GetDefaultStrategyConfig(lang string) StrategyConfig {
    config := StrategyConfig{
        // ... 现有配置 ...
        Indicators: IndicatorConfig{
            // ... 现有指标配置 ...
            EnableBB:   false,
            BBPeriod:   20,
            BBStdDev:   2.0,
        },
    }
    // ... 其他代码 ...
}
```

---

## 前端添加自定义指标

### 步骤 1: 更新 TypeScript 类型定义

在 `web/src/types.ts` 的 `IndicatorConfig` 接口中添加字段：

```typescript
export interface IndicatorConfig {
  // ... 现有字段 ...
  
  // Bollinger Bands
  enable_bb?: boolean;
  bb_period?: number;      // 默认 20
  bb_std_dev?: number;    // 默认 2.0
}
```

### 步骤 2: 更新 IndicatorEditor 组件

在 `web/src/components/strategy/IndicatorEditor.tsx` 中添加 UI：

#### 2.1 添加翻译文本

```typescript
const t = (key: string) => {
  const translations: Record<string, Record<string, string>> = {
    // ... 现有翻译 ...
    
    // 新增指标翻译
    bb: { zh: '布林带', en: 'Bollinger Bands' },
    bbDesc: { zh: '布林带指标，显示价格波动范围', en: 'Bollinger Bands, shows price volatility range' },
    bbPeriod: { zh: '周期', en: 'Period' },
    bbStdDev: { zh: '标准差倍数', en: 'Std Dev Multiplier' },
  }
  return translations[key]?.[language] || key
}
```

#### 2.2 在指标网格中添加新指标

找到指标网格部分（约第 292 行），添加新指标：

```typescript
{[
  { key: 'enable_ema', label: 'ema', desc: 'emaDesc', color: '#F0B90B', periodKey: 'ema_periods', defaultPeriods: '20,50' },
  { key: 'enable_macd', label: 'macd', desc: 'macdDesc', color: '#a855f7' },
  { key: 'enable_rsi', label: 'rsi', desc: 'rsiDesc', color: '#F6465D', periodKey: 'rsi_periods', defaultPeriods: '7,14' },
  { key: 'enable_atr', label: 'atr', desc: 'atrDesc', color: '#60a5fa', periodKey: 'atr_periods', defaultPeriods: '14' },
  // 新增：Bollinger Bands
  { key: 'enable_bb', label: 'bb', desc: 'bbDesc', color: '#22c55e', periodKey: 'bb_period', defaultPeriods: '20' },
].map(({ key, label, desc, color, periodKey, defaultPeriods }) => (
  // ... 现有渲染逻辑 ...
))}
```

#### 2.3 添加参数输入（如果需要）

如果指标需要额外参数（如标准差倍数），可以在指标卡片中添加：

```typescript
{periodKey && config[key as keyof IndicatorConfig] && (
  <div className="space-y-1">
    {/* 周期参数 */}
    <input
      type="text"
      value={(config[periodKey as keyof IndicatorConfig] as number) || defaultPeriods}
      onChange={(e) => {
        if (disabled) return
        const period = parseInt(e.target.value.trim()) || parseInt(defaultPeriods)
        onChange({ ...config, [periodKey]: period })
      }}
      disabled={disabled}
      placeholder={defaultPeriods}
      className="w-full px-2 py-1 rounded text-[10px] text-center"
      style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
    />
    
    {/* 如果是 BB，添加标准差倍数输入 */}
    {key === 'enable_bb' && (
      <input
        type="number"
        value={config.bb_std_dev || 2.0}
        onChange={(e) => {
          if (disabled) return
          const stdDev = parseFloat(e.target.value) || 2.0
          onChange({ ...config, bb_std_dev: stdDev })
        }}
        disabled={disabled}
        step="0.1"
        min="0.5"
        max="5.0"
        className="w-full px-2 py-1 rounded text-[10px] text-center"
        style={{ background: '#1E2329', border: '1px solid #2B3139', color: '#EAECEF' }}
        placeholder="2.0"
      />
    )}
  </div>
)}
```

---

## 完整示例：添加 Bollinger Bands (BB)

### 后端完整实现

#### 1. `market/data.go` - 添加计算函数

```go
// calculateBollingerBands 计算布林带
func calculateBollingerBands(klines []Kline, period int, stdDev float64) (float64, float64, float64) {
    if len(klines) < period {
        return 0, 0, 0
    }
    
    // 计算 SMA (middle band)
    sum := 0.0
    startIdx := len(klines) - period
    for i := startIdx; i < len(klines); i++ {
        sum += klines[i].Close
    }
    middleBand := sum / float64(period)
    
    // 计算标准差
    variance := 0.0
    for i := startIdx; i < len(klines); i++ {
        diff := klines[i].Close - middleBand
        variance += diff * diff
    }
    stdDeviation := math.Sqrt(variance / float64(period))
    
    // 计算上下轨
    upperBand := middleBand + (stdDeviation * stdDev)
    lowerBand := middleBand - (stdDeviation * stdDev)
    
    return upperBand, middleBand, lowerBand
}
```

#### 2. `market/types.go` - 更新数据结构

```go
type Data struct {
    // ... 现有字段 ...
    CurrentBBUpper  float64
    CurrentBBMiddle float64
    CurrentBBLower  float64
}

type TimeframeSeriesData struct {
    // ... 现有字段 ...
    BBUpperValues  []float64 `json:"bb_upper_values"`
    BBMiddleValues []float64 `json:"bb_middle_values"`
    BBLowerValues  []float64 `json:"bb_lower_values"`
}
```

#### 3. `store/strategy.go` - 更新配置

```go
type IndicatorConfig struct {
    // ... 现有字段 ...
    EnableBB   bool    `json:"enable_bb"`
    BBPeriod   int     `json:"bb_period,omitempty"`
    BBStdDev   float64 `json:"bb_std_dev,omitempty"`
}
```

### 前端完整实现

#### 1. `web/src/types.ts`

```typescript
export interface IndicatorConfig {
  // ... 现有字段 ...
  enable_bb?: boolean;
  bb_period?: number;
  bb_std_dev?: number;
}
```

#### 2. `web/src/components/strategy/IndicatorEditor.tsx`

在指标数组中添加：
```typescript
{ key: 'enable_bb', label: 'bb', desc: 'bbDesc', color: '#22c55e', periodKey: 'bb_period', defaultPeriods: '20' }
```

---

## 测试和验证

### 1. 后端单元测试

创建 `market/data_test.go` 测试文件：

```go
func TestCalculateBollingerBands(t *testing.T) {
    // 创建测试数据
    klines := []Kline{
        {Close: 100}, {Close: 102}, {Close: 101}, {Close: 103},
        {Close: 105}, {Close: 104}, {Close: 106}, {Close: 108},
        {Close: 107}, {Close: 109}, {Close: 110}, {Close: 112},
        {Close: 111}, {Close: 113}, {Close: 115}, {Close: 114},
        {Close: 116}, {Close: 118}, {Close: 117}, {Close: 119},
    }
    
    upper, middle, lower := calculateBollingerBands(klines, 20, 2.0)
    
    if middle == 0 {
        t.Error("Middle band should not be zero")
    }
    if upper <= middle {
        t.Error("Upper band should be greater than middle band")
    }
    if lower >= middle {
        t.Error("Lower band should be less than middle band")
    }
}
```

### 2. 集成测试

1. **启动后端服务**
   ```bash
   go run main.go
   ```

2. **测试 API 端点**
   ```bash
   # 创建包含新指标的策略配置
   curl -X POST http://localhost:8080/api/strategies \
     -H "Content-Type: application/json" \
     -d '{
       "name": "Test Strategy",
       "config": {
         "indicators": {
           "enable_bb": true,
           "bb_period": 20,
           "bb_std_dev": 2.0
         }
       }
     }'
   ```

3. **验证数据输出**
   - 检查 `formatMarketData()` 输出是否包含 BB 数据
   - 检查 AI 提示词中是否包含 BB 指标

### 3. 前端测试

1. **启动前端开发服务器**
   ```bash
   cd web
   npm run dev
   ```

2. **测试 UI**
   - 打开策略配置页面
   - 启用新指标
   - 设置参数
   - 保存并验证配置是否正确提交

---

## 常见问题

### Q1: 指标计算函数应该放在哪里？

**A:** 所有指标计算函数都应该放在 `market/data.go` 文件中，保持代码集中管理。

### Q2: 如何支持多个周期的指标（如 EMA20 和 EMA50）？

**A:** 
- 在配置中使用数组类型：`EMAPeriods []int`
- 在数据结构中为每个周期创建独立字段：`EMA20Values`, `EMA50Values`
- 在计算函数中循环处理每个周期

### Q3: 指标值应该存储在哪里？

**A:**
- **当前值**：存储在 `market.Data` 结构体中（如 `CurrentEMA20`）
- **时间序列值**：存储在 `market.TimeframeSeriesData` 结构体中（如 `EMA20Values []float64`）

### Q4: 如何确保指标计算的性能？

**A:**
- 使用滑动窗口算法避免重复计算
- 缓存中间结果
- 对于复杂指标，考虑使用增量更新

### Q5: 前端和后端的配置字段命名不一致怎么办？

**A:** 
- 后端使用 `snake_case`（如 `enable_bb`）
- 前端 TypeScript 也使用 `snake_case` 以保持一致性
- JSON 序列化时会自动处理

### Q6: 如何添加需要多个参数的指标？

**A:**
- 在 `IndicatorConfig` 中添加多个字段
- 在前端 UI 中添加多个输入框
- 在计算函数中接收多个参数

### Q7: 指标计算失败时应该返回什么值？

**A:**
- 返回 `0` 或 `NaN` 表示计算失败
- 在格式化输出时检查并跳过无效值
- 记录警告日志以便调试

---

## 总结

添加自定义技术指标的完整流程：

1. ✅ **后端计算函数** - 在 `market/data.go` 实现计算逻辑
2. ✅ **数据结构更新** - 在 `market/types.go` 添加字段
3. ✅ **配置结构更新** - 在 `store/strategy.go` 添加配置字段
4. ✅ **格式化输出** - 在 `decision/engine.go` 添加格式化逻辑
5. ✅ **前端类型定义** - 在 `web/src/types.ts` 添加 TypeScript 类型
6. ✅ **前端 UI** - 在 `IndicatorEditor.tsx` 添加配置界面
7. ✅ **测试验证** - 编写单元测试和集成测试

遵循以上步骤，即可成功添加新的技术指标到 NOFX 系统中。

---

**文档版本**: 1.0.0  
**最后更新**: 2025-01-15  
**维护者**: NOFX Development Team
