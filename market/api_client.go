package market

import (
	"context"
	"fmt"
	"time"

	"github.com/adshao/go-binance/v2/futures"
)

const (
	baseURL = "https://fapi.binance.com"
)

var sharedAPIClient = NewAPIClient()

type APIClient struct {
	client *futures.Client
}

func NewAPIClient() *APIClient {
	return &APIClient{
		client: futures.NewClient("", ""),
	}
}

func (c *APIClient) GetExchangeInfo() (*ExchangeInfo, error) {
	resp, err := c.client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return nil, err
	}

	var exchangeInfo ExchangeInfo
	for _, symbol := range resp.Symbols {
		exchangeInfo.Symbols = append(exchangeInfo.Symbols, SymbolInfo{
			Symbol:            symbol.Symbol,
			Status:            symbol.Status,
			BaseAsset:         symbol.BaseAsset,
			QuoteAsset:        symbol.QuoteAsset,
			ContractType:      string(symbol.ContractType),
			PricePrecision:    symbol.PricePrecision,
			QuantityPrecision: symbol.QuantityPrecision,
		})
	}
	return &exchangeInfo, nil
}

func (c *APIClient) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	req := c.client.NewKlinesService()
	req.Symbol(symbol)
	req.Interval(interval)
	req.Limit(limit)
	resp, err := req.Do(context.Background())
	if err != nil {
		return nil, err
	}

	klines := make([]Kline, len(resp))
	for i, kline := range resp {
		klines[i] = Kline{
			OpenTime:            kline.OpenTime,
			Open:                parseFloat(kline.Open),
			High:                parseFloat(kline.High),
			Low:                 parseFloat(kline.Low),
			Close:               parseFloat(kline.Close),
			Volume:              parseFloat(kline.Volume),
			CloseTime:           kline.CloseTime,
			QuoteVolume:         parseFloat(kline.QuoteAssetVolume),
			Trades:              int(kline.TradeNum),
			TakerBuyBaseVolume:  parseFloat(kline.TakerBuyBaseAssetVolume),
			TakerBuyQuoteVolume: parseFloat(kline.TakerBuyQuoteAssetVolume),
		}
	}

	return klines, nil
}

func (c *APIClient) GetKlinesRange(symbol, interval string, start, end time.Time) ([]Kline, error) {
	const maxLimit = 1000 // Binance API maximum limit per request
	startMs := start.UnixMilli()
	endMs := end.UnixMilli()

	var allKlines []Kline
	ctx := context.Background()

	for {
		req := c.client.NewKlinesService()
		req.Symbol(symbol)
		req.Interval(interval)
		req.StartTime(startMs)
		req.EndTime(endMs)
		req.Limit(maxLimit)

		resp, err := req.Do(ctx)
		if err != nil {
			return nil, err
		}

		count := len(allKlines)
		for _, kline := range resp {
			item := Kline{
				OpenTime:            kline.OpenTime,
				Open:                parseFloat(kline.Open),
				High:                parseFloat(kline.High),
				Low:                 parseFloat(kline.Low),
				Close:               parseFloat(kline.Close),
				Volume:              parseFloat(kline.Volume),
				CloseTime:           kline.CloseTime,
				QuoteVolume:         parseFloat(kline.QuoteAssetVolume),
				Trades:              int(kline.TradeNum),
				TakerBuyBaseVolume:  parseFloat(kline.TakerBuyBaseAssetVolume),
				TakerBuyQuoteVolume: parseFloat(kline.TakerBuyQuoteAssetVolume),
			}

			if startMs = kline.CloseTime; startMs > endMs {
				break
			}

			allKlines = append(allKlines, item)
		}

		// If the number of klines added is less than the max limit, we've reached the end of the data
		if len(allKlines)-count < maxLimit {
			break
		}
	}

	return allKlines, nil
}

func GetKlinesRange(symbol, interval string, start, end time.Time) ([]Kline, error) {
	return sharedAPIClient.GetKlinesRange(symbol, interval, start, end)
}

func (c *APIClient) GetFundingData(symbol string) (*FundingData, error) {
	req := c.client.NewFundingRateService()
	req.Symbol(symbol)
	resp, err := req.Do(context.Background())
	if err != nil {
		return nil, err
	}

	for _, rate := range resp {
		if rate.Symbol == symbol {
			return &FundingData{
				Symbol:      symbol,
				FundingRate: parseFloat(rate.FundingRate),
				MarkPrice:   parseFloat(rate.MarkPrice),
			}, nil
		}
	}

	return nil, fmt.Errorf("funding rate not found for symbol: %s", symbol)
}

func GetFundingData(symbol string) (*FundingData, error) {
	return sharedAPIClient.GetFundingData(symbol)
}

func (c *APIClient) GetOpenInterestData(symbol string) (*OIData, error) {
	req := c.client.NewGetOpenInterestService()
	req.Symbol(symbol)
	resp, err := req.Do(context.Background())
	if err != nil {
		return nil, err
	}

	oi := parseFloat(resp.OpenInterest)
	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // Approximate average
	}, nil
}
