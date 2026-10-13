package cse

import (
	"context"
	"net/url"
)

// SymbolInfo is the per-security detail in companyInfoSummery.
type SymbolInfo struct {
	StockID           int64    `json:"id"`
	Symbol            string   `json:"symbol"`
	Name              string   `json:"name"`
	ISIN              string   `json:"isin"`
	IssueDate         string   `json:"issueDate"`
	QuantityIssued    float64  `json:"quantityIssued"`
	ParValue          float64  `json:"parValue"`
	LastTradedPrice   float64  `json:"lastTradedPrice"`
	PreviousClose     float64  `json:"previousClose"`
	ClosingPrice      float64  `json:"closingPrice"`
	Change            float64  `json:"change"`
	ChangePercentage  float64  `json:"changePercentage"`
	High              float64  `json:"hiTrade"`
	Low               float64  `json:"lowTrade"`
	MarketCap         float64  `json:"marketCap"`
	MarketCapPct      float64  `json:"marketCapPercentage"`
	WTDHigh           float64  `json:"wtdHiPrice"`
	WTDLow            float64  `json:"wtdLowPrice"`
	MTDHigh           float64  `json:"mtdHiPrice"`
	MTDLow            float64  `json:"mtdLowPrice"`
	YTDHigh           float64  `json:"ytdHiPrice"`
	YTDLow            float64  `json:"ytdLowPrice"`
	High52W           float64  `json:"p12HiPrice"`
	Low52W            float64  `json:"p12LowPrice"`
	AllTimeHigh       float64  `json:"allHiPrice"`
	AllTimeLow        float64  `json:"allLowPrice"`
	TodayShareVolume  float64  `json:"tdyShareVolume"`
	YTDShareVolume    float64  `json:"ytdShareVolume"`
	ShareVolume52W    float64  `json:"p12ShareVolume"`
	TodayTurnover     float64  `json:"tdyTurnover"`
	YTDTurnover       float64  `json:"ytdTurnover"`
	ForeignHoldings   *float64 `json:"foreignHoldings"`
	ForeignPercentage *float64 `json:"foreignPercentage"`
}

// Beta is the CSE-published beta for a security.
type Beta struct {
	SecurityID int64   `json:"securityId"`
	ASPI       float64 `json:"triASIBetaValue"`
	SPSL20     float64 `json:"betaValueSPSL"`
	Period     string  `json:"triASIBetaPeriod"`
	Quarter    int     `json:"quarter"`
}

// CompanyInfo is the companyInfoSummery response.
type CompanyInfo struct {
	Info SymbolInfo `json:"reqSymbolInfo"`
	Beta *Beta      `json:"reqSymbolBetaInfo"`
	Logo *Logo      `json:"reqLogo"`
}

// CompanyInfoSummary fetches the quote, ranges and beta for a full security
// code such as "JKH.N0000" (a bare ticker returns ErrNoData).
func (c *Client) CompanyInfoSummary(ctx context.Context, code string) (*CompanyInfo, error) {
	var ci CompanyInfo
	if err := c.post(ctx, "companyInfoSummery", url.Values{"symbol": {code}}, &ci); err != nil {
		return nil, err
	}
	if ci.Info.Symbol == "" {
		return nil, ErrNoData
	}
	return &ci, nil
}
