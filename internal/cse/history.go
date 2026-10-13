package cse

import (
	"context"
	"net/url"
	"strconv"
)

// Chart periods accepted by companyChartDataByStock and chartData.
const (
	PeriodIntraday = 1
	PeriodWeek     = 2
	PeriodMonth    = 3
	PeriodQuarter  = 4
	PeriodYear     = 5 // daily bars, ~240 rows
)

// Bar is one price point. For daily periods T is 00:00 Asia/Colombo of the trade date.
type Bar struct {
	T      int64    `json:"t"` // ms
	Close  float64  `json:"p"`
	High   *float64 `json:"h"`
	Low    *float64 `json:"l"`
	Open   *float64 `json:"o"`
	Volume float64  `json:"q"`
}

// ChartByStock fetches price history for a numeric stockId (see SymbolInfo.StockID).
func (c *Client) ChartByStock(ctx context.Context, stockID int64, period int) ([]Bar, error) {
	var r struct {
		Bars []Bar `json:"chartData"`
	}
	form := url.Values{"stockId": {strconv.FormatInt(stockID, 10)}, "period": {strconv.Itoa(period)}}
	if err := c.post(ctx, "companyChartDataByStock", form, &r); err != nil {
		return nil, err
	}
	if len(r.Bars) == 0 {
		return nil, ErrNoData
	}
	return r.Bars, nil
}
