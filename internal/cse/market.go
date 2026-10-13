package cse

import (
	"context"
	"fmt"
	"net/url"
)

// MarketStatus returns the market state, e.g. "Regular Trading" or "Market Closed".
func (c *Client) MarketStatus(ctx context.Context) (string, error) {
	var s struct {
		Status string `json:"status"`
	}
	if err := c.post(ctx, "marketStatus", url.Values{}, &s); err != nil {
		return "", err
	}
	return s.Status, nil
}

// MarketSummary is today's market-wide activity (marketSummery).
type MarketSummary struct {
	ID          int64   `json:"id"`
	Turnover    float64 `json:"tradeVolume"` // LKR, despite the name
	ShareVolume float64 `json:"shareVolume"`
	TradeDate   int64   `json:"tradeDate"` // ms
	Trades      float64 `json:"trades"`
}

// MarketSummary fetches today's turnover, share volume and trade count.
func (c *Client) MarketSummary(ctx context.Context) (*MarketSummary, error) {
	var s MarketSummary
	if err := c.post(ctx, "marketSummery", url.Values{}, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// IndexValue is the current value of an index (aspiData, snpData).
type IndexValue struct {
	ID         int64   `json:"id"`
	Value      float64 `json:"value"`
	Low        float64 `json:"lowValue"`
	High       float64 `json:"highValue"`
	Change     float64 `json:"change"`
	Percentage float64 `json:"percentage"`
	SectorID   float64 `json:"sectorId"`  // 1 = ASPI, 40 = S&P SL20
	Timestamp  int64   `json:"timestamp"` // ms
}

// ASPI fetches the All Share Price Index.
func (c *Client) ASPI(ctx context.Context) (*IndexValue, error) {
	return c.index(ctx, "aspiData")
}

// SNP fetches the S&P Sri Lanka 20 index.
func (c *Client) SNP(ctx context.Context) (*IndexValue, error) {
	return c.index(ctx, "snpData")
}

func (c *Client) index(ctx context.Context, endpoint string) (*IndexValue, error) {
	var v IndexValue
	if err := c.post(ctx, endpoint, url.Values{}, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// TradeSummary is one security in today's trade summary.
type TradeSummary struct {
	StockID          int64   `json:"id"`
	Name             string  `json:"name"`
	Symbol           string  `json:"symbol"`
	Quantity         float64 `json:"quantity"`
	PercentageChange float64 `json:"percentageChange"`
	Change           float64 `json:"change"`
	Price            float64 `json:"price"`
	PreviousClose    float64 `json:"previousClose"`
	High             float64 `json:"high"`
	Low              float64 `json:"low"`
	Open             float64 `json:"open"`
	ClosingPrice     float64 `json:"closingPrice"`
	LastTradedTime   int64   `json:"lastTradedTime"` // ms
	IssueDate        *string `json:"issueDate"`
	Turnover         float64 `json:"turnover"`
	ShareVolume      float64 `json:"sharevolume"`
	TradeVolume      float64 `json:"tradevolume"`
	MarketCap        float64 `json:"marketCap"`
	MarketCapPct     float64 `json:"marketCapPercentage"`
	CrossingVolume   float64 `json:"crossingVolume"`
	CrossingTrades   float64 `json:"crossingTradeVol"`
	Status           int     `json:"status"`
}

// TradeSummary fetches today's prices and volumes for every traded security.
func (c *Client) TradeSummary(ctx context.Context) ([]TradeSummary, error) {
	var r struct {
		Rows []TradeSummary `json:"reqTradeSummery"`
	}
	if err := c.post(ctx, "tradeSummary", url.Values{}, &r); err != nil {
		return nil, err
	}
	return r.Rows, nil
}

// Sector is one S&P/CSE GICS industry-group index (allSectors).
type Sector struct {
	ID            int64    `json:"id"`
	SectorID      int64    `json:"sectorId"`
	Symbol        string   `json:"symbol"`
	IndexCode     *string  `json:"indexCode"`
	IndexCodeSP   string   `json:"indexCodeSp"`
	IndexName     string   `json:"indexName"`
	Name          string   `json:"name"`
	Value         float64  `json:"indexValue"`
	Change        float64  `json:"change"`
	Percentage    float64  `json:"percentage"`
	Trades        *float64 `json:"sectorTradeToday"`
	Volume        *float64 `json:"sectorVolumeToday"`
	Turnover      *float64 `json:"sectorTurnoverToday"`
	PreviousClose float64  `json:"sectorPreviousClose"`
	Time          int64    `json:"transactionTime"` // ms
}

// Sectors fetches today's sector index values.
func (c *Client) Sectors(ctx context.Context) ([]Sector, error) {
	var s []Sector
	if err := c.post(ctx, "allSectors", url.Values{}, &s); err != nil {
		return nil, err
	}
	return s, nil
}

// DailyMarket is one day of market statistics (dailyMarketSummery).
type DailyMarket struct {
	ID                int64    `json:"id"`
	TradeDate         int64    `json:"tradeDate"` // ms, 00:00 Colombo
	MarketTurnover    float64  `json:"marketTurnover"`
	MarketTrades      float64  `json:"marketTrades"`
	EquityTurnover    float64  `json:"equityTurnover"`
	DomesticPurchase  float64  `json:"equityDomesticPurchase"`
	DomesticSales     float64  `json:"equityDomesticSales"`
	ForeignPurchase   float64  `json:"equityForeignPurchase"`
	ForeignSales      float64  `json:"equityForeignSales"`
	ShareVolume       float64  `json:"volumeOfTurnOverNumber"`
	Trades            float64  `json:"tradesNo"`
	ListedCompanies   float64  `json:"listedCompanyNumber"`
	TradedCompanies   float64  `json:"tradeCompanyNumber"`
	MarketCap         float64  `json:"marketCap"`
	PER               float64  `json:"per"`
	PBV               float64  `json:"pbv"`
	DY                float64  `json:"dy"`
	ASI               float64  `json:"asi"`
	SPSL20            float64  `json:"spp"`
	TRIASI            float64  `json:"triasi"`
	TRIMPI            *float64 `json:"trimpi"`
	MPI               *float64 `json:"mpi"`
	CDSForeignHolding float64  `json:"cdsMarketValueForeign"`
}

// DailyMarketSummary fetches the recent daily market statistics, newest first.
func (c *Client) DailyMarketSummary(ctx context.Context) ([]DailyMarket, error) {
	var nested [][]DailyMarket
	if err := c.post(ctx, "dailyMarketSummery", url.Values{}, &nested); err != nil {
		return nil, err
	}
	var days []DailyMarket
	for _, d := range nested {
		days = append(days, d...)
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("request dailyMarketSummery: %w", ErrNoData)
	}
	return days, nil
}
