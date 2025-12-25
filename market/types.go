package market

// Data market data structure
type Data struct {
	Symbol            string          `json:"symbol"`
	CurrentPrice      float64         `json:"current_price"`
	PriceChange1h     float64         `json:"price_change_1h"`
	PriceChange4h     float64         `json:"price_change_4h"`
	CurrentEMA20      float64         `json:"current_ema20"`
	CurrentMACD       float64         `json:"current_macd"`
	CurrentRSI7       float64         `json:"current_rsi7"`
	OpenInterest      *OIData         `json:"open_interest"`
	FundingRate       float64         `json:"funding_rate"`
	IntradaySeries    *IntradayData   `json:"intraday_series"`
	LongerTermContext *LongerTermData `json:"longer_term_context"`
	// Multi-timeframe data (new)
	TimeframeData map[string]*TimeframeSeriesData `json:"timeframe_data,omitempty"`
}

// KlineBar single kline bar with OHLCV data
type KlineBar struct {
	Time   int64   `json:"time"`   // Unix timestamp in milliseconds
	Open   float64 `json:"open"`   // Open price
	High   float64 `json:"high"`   // High price
	Low    float64 `json:"low"`    // Low price
	Close  float64 `json:"close"`  // Close price
	Volume float64 `json:"volume"` // Volume
}

// TimeframeSeriesData series data for a single timeframe
type TimeframeSeriesData struct {
	Timeframe   string     `json:"timeframe"`    // Timeframe identifier, e.g. "5m", "15m", "1h"
	Klines      []KlineBar `json:"klines"`       // Full OHLCV kline data
	MidPrices   []float64  `json:"mid_prices"`   // Price series (deprecated, kept for compatibility)
	EMA20Values []float64  `json:"ema20_values"` // EMA20 series
	EMA50Values []float64  `json:"ema50_values"` // EMA50 series
	MACDValues  []float64  `json:"macd_values"`  // MACD series
	RSI7Values  []float64  `json:"rsi7_values"`  // RSI7 series
	RSI14Values []float64  `json:"rsi14_values"` // RSI14 series
	Volume      []float64  `json:"volume"`       // Volume series (deprecated, use Klines)
	ATR14       float64    `json:"atr14"`        // ATR14
	// Bollinger Bands (period 20, std dev multiplier 2)
	BOLLUpper  []float64 `json:"boll_upper"`  // Upper band
	BOLLMiddle []float64 `json:"boll_middle"` // Middle band (SMA)
	BOLLLower  []float64 `json:"boll_lower"`  // Lower band
}

type FundingData struct {
	Symbol      string
	FundingRate float64
	MarkPrice   float64
}

// OIData Open Interest data
type OIData struct {
	Latest  float64
	Average float64
}

// IntradayData intraday data (3-minute interval)
type IntradayData struct {
	MidPrices   []float64
	EMA20Values []float64
	MACDValues  []float64
	RSI7Values  []float64
	RSI14Values []float64
	Volume      []float64
	ATR14       float64
}

// LongerTermData longer-term data (4-hour timeframe)
type LongerTermData struct {
	EMA20         float64
	EMA50         float64
	ATR3          float64
	ATR14         float64
	CurrentVolume float64
	AverageVolume float64
	MACDValues    []float64
	RSI14Values   []float64
}

// Binance API response structure
type ExchangeInfo struct {
	Symbols []SymbolInfo `json:"symbols"`
}

type SymbolInfo struct {
	Symbol            string `json:"symbol"`
	Status            string `json:"status"`
	BaseAsset         string `json:"baseAsset"`
	QuoteAsset        string `json:"quoteAsset"`
	ContractType      string `json:"contractType"`
	PricePrecision    int    `json:"pricePrecision"`
	QuantityPrecision int    `json:"quantityPrecision"`
}

type Kline struct {
	OpenTime            int64   `json:"openTime"`
	Open                float64 `json:"open"`
	High                float64 `json:"high"`
	Low                 float64 `json:"low"`
	Close               float64 `json:"close"`
	Volume              float64 `json:"volume"`
	CloseTime           int64   `json:"closeTime"`
	QuoteVolume         float64 `json:"quoteVolume"`
	Trades              int     `json:"trades"`
	TakerBuyBaseVolume  float64 `json:"takerBuyBaseVolume"`
	TakerBuyQuoteVolume float64 `json:"takerBuyQuoteVolume"`
}

type PriceTicker struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type Ticker24hr struct {
	Symbol             string `json:"symbol"`
	PriceChange        string `json:"priceChange"`
	PriceChangePercent string `json:"priceChangePercent"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
}
