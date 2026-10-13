package cse

import (
	"context"
	"net/url"
)

// Announcement is a corporate disclosure from approvedAnnouncement. The list
// endpoint leaves Symbol and the date fields null, so match on Company.
type Announcement struct {
	ID             int64   `json:"id"`
	AnnouncementID int64   `json:"announcementId"`
	CreatedDate    int64   `json:"createdDate"`        // ms
	Date           string  `json:"dateOfAnnouncement"` // "12 Oct 2026"
	Category       string  `json:"announcementCategory"`
	Company        string  `json:"company"`
	Symbol         *string `json:"symbol"`
	Type           *string `json:"type"`
	XD             *string `json:"xd"`
	XR             *string `json:"xr"`
	RecordDate     *string `json:"recordDate"`
	PaymentDate    *string `json:"paymentDate"`
	AGMDate        *string `json:"agmDate"`
	Remarks        *string `json:"remarks"`
}

// Announcements fetches the most recent approved corporate announcements.
func (c *Client) Announcements(ctx context.Context) ([]Announcement, error) {
	var r struct {
		Rows []Announcement `json:"approvedAnnouncements"`
	}
	if err := c.post(ctx, "approvedAnnouncement", url.Values{}, &r); err != nil {
		return nil, err
	}
	return r.Rows, nil
}

// FinancialAnnouncement is a newly uploaded financial report, market-wide.
// Its dates are strings ("12 Oct 2026 06:05:35 PM"), unlike Report.
type FinancialAnnouncement struct {
	ID             int64   `json:"id"`
	Path           string  `json:"path"`
	ManualDate     int64   `json:"manualDate"` // ms
	UploadedDate   string  `json:"uploadedDate"`
	AuthorizedDate *string `json:"authorizedDate"`
	FileText       string  `json:"fileText"`
	Name           string  `json:"name"`
	Symbol         string  `json:"symbol"` // ticker only, e.g. "ASPH"
}

// FinancialAnnouncements fetches the latest report uploads across the market.
func (c *Client) FinancialAnnouncements(ctx context.Context) ([]FinancialAnnouncement, error) {
	var r struct {
		Rows []FinancialAnnouncement `json:"reqFinancialAnnouncemnets"`
	}
	if err := c.post(ctx, "getFinancialAnnouncement", url.Values{}, &r); err != nil {
		return nil, err
	}
	return r.Rows, nil
}
