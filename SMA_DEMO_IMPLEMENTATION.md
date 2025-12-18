# SMA (Simple Moving Average) 指标实现 Demo

本文档展示了如何在 NOFX 项目中完整实现一个简单的技术指标 - SMA (简单移动平均线)。

## 实现概览

SMA 是最简单的移动平均线指标，计算公式为：`SMA = (最近N根K线的收盘价之和) / N`

## 后端实现

### 1. 计算函数 (`market/data.go`)

```go
// calculateSMA calculates Simple Moving Average
func calculateSMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// Calculate SMA: sum of last N closes / N
	sum := 0.0
	startIdx := len(klines) - period
	for i := startIdx; i < len(klines); i++ {
		sum += klines[i].Close
	}

	return sum / float64(period)
}
```

### 2. 数据结构更新 (`market/types.go`)

```go
// Data 结构体添加字段
type Data struct {
    // ... 现有字段 ...
    CurrentSMA20 float64 // SMA 20-period (demo indicator)
}

// TimeframeSeriesData 结构体添加字段
type TimeframeSeriesData struct {
    // ... 现有字段 ...
    SMA20Values []float64 `json:"sma20_values"` // SMA20 series
}

// IntradayData 结构体添加字段
type IntradayData struct {
    // ... 现有字段 ...
    SMA20Values []float64 // Demo: SMA 20 series
}
```

### 3. 配置结构更新 (`store/strategy.go`)

```go
type IndicatorConfig struct {
    // ... 现有字段 ...
    EnableSMA bool `json:"enable_sma"`         // Demo: Simple Moving Average
    SMAPeriods []int `json:"sma_periods,omitempty"` // default [20]
}
```

### 4. 在数据获取函数中计算 (`market/data.go`)

在 `GetWithTimeframes()` 函数中：
```go
currentSMA20 := calculateSMA(primaryKlines, 20) // Demo: SMA 20-period

return &Data{
    // ... 其他字段 ...
    CurrentSMA20: currentSMA20,
    // ...
}
```

在 `calculateTimeframeSeries()` 函数中：
```go
// Demo: Calculate SMA20 for each point
if i >= 19 {
    sma20 := calculateSMA(klines[:i+1], 20)
    data.SMA20Values = append(data.SMA20Values, sma20)
}
```

### 5. 格式化输出 (`decision/engine.go`)

在 `formatMarketData()` 函数中：
```go
// Demo: SMA indicator formatting
if indicators.EnableSMA {
    sb.WriteString(fmt.Sprintf(", current_sma20 = %.3f", data.CurrentSMA20))
}
```

在 `formatTimeframeSeriesData()` 函数中：
```go
// Demo: SMA indicator formatting
if indicators.EnableSMA {
    if len(data.SMA20Values) > 0 {
        sb.WriteString(fmt.Sprintf("SMA20: %s\n", formatFloatSlice(data.SMA20Values)))
    }
}
```

## 前端实现

### 1. TypeScript 类型定义 (`web/src/types.ts`)

```typescript
export interface IndicatorConfig {
  // ... 现有字段 ...
  enable_sma?: boolean; // Demo: Simple Moving Average
  sma_periods?: number[]; // Demo: SMA periods, default [20]
}
```

### 2. UI 组件更新 (`web/src/components/strategy/IndicatorEditor.tsx`)

#### 添加翻译文本：
```typescript
sma: { zh: 'SMA 均线', en: 'SMA' },
smaDesc: { zh: '简单移动平均线 (Demo)', en: 'Simple Moving Average (Demo)' },
```

#### 在指标数组中添加：
```typescript
{[
  // ... 现有指标 ...
  { key: 'enable_sma', label: 'sma', desc: 'smaDesc', color: '#22c55e', periodKey: 'sma_periods', defaultPeriods: '20' },
].map(...)}
```

## 使用示例

### 1. 在策略配置中启用 SMA

```json
{
  "indicators": {
    "enable_sma": true,
    "sma_periods": [20]
  }
}
```

### 2. AI 提示词中的输出格式

当启用 SMA 后，AI 会看到类似这样的数据：

```
current_price = 50000.0000, current_sma20 = 49850.500

=== 5M TIMEFRAME ===
SMA20: [49800.0, 49820.5, 49840.2, 49850.5]
```

## 测试

### 后端测试

```go
func TestCalculateSMA(t *testing.T) {
    klines := []Kline{
        {Close: 100}, {Close: 102}, {Close: 101}, {Close: 103},
        {Close: 105}, {Close: 104}, {Close: 106}, {Close: 108},
        {Close: 107}, {Close: 109}, {Close: 110}, {Close: 112},
        {Close: 111}, {Close: 113}, {Close: 115}, {Close: 114},
        {Close: 116}, {Close: 118}, {Close: 117}, {Close: 119},
    }
    
    sma := calculateSMA(klines, 20)
    
    // 验证 SMA 计算正确
    expected := (100 + 102 + 101 + 103 + 105 + 104 + 106 + 108 + 
                 107 + 109 + 110 + 112 + 111 + 113 + 115 + 114 + 
                 116 + 118 + 117 + 119) / 20.0
    
    if math.Abs(sma - expected) > 0.01 {
        t.Errorf("SMA calculation error: got %.2f, expected %.2f", sma, expected)
    }
}
```

## 文件修改清单

### 后端文件
- ✅ `market/data.go` - 添加 `calculateSMA()` 函数，在数据获取函数中调用
- ✅ `market/types.go` - 在 `Data`, `TimeframeSeriesData`, `IntradayData` 中添加 SMA 字段
- ✅ `store/strategy.go` - 在 `IndicatorConfig` 中添加 `EnableSMA` 和 `SMAPeriods` 配置
- ✅ `decision/engine.go` - 在格式化函数中添加 SMA 输出

### 前端文件
- ✅ `web/src/types.ts` - 添加 `enable_sma` 和 `sma_periods` 类型定义
- ✅ `web/src/components/strategy/IndicatorEditor.tsx` - 添加 SMA UI 组件和翻译

## 总结

这个 SMA 实现展示了添加自定义指标的完整流程：

1. ✅ 后端计算函数
2. ✅ 数据结构更新
3. ✅ 配置结构更新
4. ✅ 格式化输出
5. ✅ 前端类型定义
6. ✅ 前端 UI 组件

所有代码已经实现并集成到系统中，可以直接使用！
