package cse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Text is a JSON value the CSE sends as a string in some rows and a number in
// others (consideration, proportions, share counts). It keeps the text.
type Text string

// UnmarshalJSON accepts a string, a number or null.
func (t *Text) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case string(b) == "null":
		*t = ""
	case len(b) > 0 && b[0] == '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*t = Text(strings.TrimSpace(s))
	default:
		*t = Text(string(b)) // a number: keep its text
	}
	return nil
}

// Float parses the text as a number.
func (t Text) Float() (float64, bool) {
	v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(string(t)), ",", ""), 64)
	return v, err == nil
}

// SMDAnnouncement is one row of the website's announcement search (/api/smd).
// Unlike approvedAnnouncement it carries the dates and proportions. Dates are
// DD-MM-YYYY. Only the fields hex uses are decoded.
type SMDAnnouncement struct {
	ID       int64  `json:"id"` // the announcement id (announcementId in approvedAnnouncement)
	SecID    int64  `json:"secId"`
	Company  string `json:"company"`
	Symbol   string `json:"symbol"` // ticker, without the series
	Category string `json:"announcementCategory"`
	Date     string `json:"dateOfAnnouncement"`
	Remarks  string `json:"remarks"`

	// Dividends and scrip dividends.
	XD                 string `json:"xd"`
	RecordDate         string `json:"recordDate"`
	Payment            string `json:"payment"`
	AGM                string `json:"agm"`
	VotingDivPerShare  Text   `json:"votingDivPerShare"`
	NonVotingDivPerShr Text   `json:"nonVotingDivPerShare"`
	VotingProportion   Text   `json:"votingPropotion"` // sic
	VotingConsider     Text   `json:"votingConsideration"`

	// Capitalisation of reserves (bonus).
	XC            string `json:"xc"`
	DateAllotment string `json:"dateAllotment"`

	// Rights issues.
	XR                string `json:"xr"`
	Allotment         string `json:"allotment"`
	RightsProportion  Text   `json:"votingShrsPropToBeIssued"`
	RightsPrice       Text   `json:"votingShareConsideration"`
	RightsShares      Text   `json:"numOfVotingShrsIssued"`
	TradingCommence   string `json:"tradingCommencement"`
	TradingSuspended  string `json:"tradingSuspended"`
	EntitlementAsAt   string `json:"subDivisionBasedOnShareholdingsAsAt"` // sub-division entitlement date
	ChildAnnouncement int64  `json:"childAnnouncement"`
}

// SMDQuery filters an announcement search. The server needs at least one
// company id and one category.
type SMDQuery struct {
	CompanyIDs []int64  `json:"companyIds"`
	Categories []string `json:"categories"`
	FromDate   string   `json:"fromDate,omitempty"` // YYYY-MM-DD; without it only the latest day is searched
	ToDate     string   `json:"toDate,omitempty"`
}

// SMDCategories are the announcement categories that can move a share price
// or carry a dividend date. "RIGHTS ISSUE (DATES)" is left out on purpose:
// the server answers it with HTTP 500 "Database error" for every query tried
// on 2026-11-02, and the same dates arrive on later "RIGHTS ISSUE" rows.
var SMDCategories = []string{
	"CASH DIVIDEND", "CASH DIVIDEND (DATES TO BE NOTIFIED)", "SCRIP DIVIDEND", "SCRIP DIVIDEND (DATES)",
	"SUB-DIVISION OF SHARES", "SUB-DIVISION OF SHARES (DATES)", "CAPITALIZATION OF RESERVES", "RIGHTS ISSUE",
}

// SMDMaxCompanies is how many company ids one search takes. Larger searches
// were answered with HTTP 500.
const SMDMaxCompanies = 20

// SecurityIDs maps a ticker to the numeric security id the announcement search
// takes. It excludes deleted securities. The id is not the stockId used for
// charts.
func (c *Client) SecurityIDs(ctx context.Context) (map[string]int64, error) {
	body, err := c.getAPI(ctx, "cntSecurity")
	if err != nil {
		return nil, err
	}
	var r struct {
		Content []struct {
			SecurityID int64  `json:"securityId"`
			Symbol     string `json:"symbol"`
			Deleted    int    `json:"deleted"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode cntSecurity: %w (body: %q)", err, snippet(body))
	}
	out := make(map[string]int64, len(r.Content))
	for _, s := range r.Content {
		if s.Deleted == 0 && s.Symbol != "" {
			out[strings.ToUpper(s.Symbol)] = s.SecurityID
		}
	}
	if len(out) == 0 {
		return nil, ErrNoData
	}
	return out, nil
}

// SearchAnnouncements runs one announcement search and returns the rows of
// every category found, in category order of the response.
func (c *Client) SearchAnnouncements(ctx context.Context, q SMDQuery) ([]SMDAnnouncement, error) {
	var r struct {
		Announcement map[string][]SMDAnnouncement `json:"Announcement"`
	}
	if err := c.postJSON(ctx, "smd", q, &r); err != nil {
		return nil, err
	}
	var out []SMDAnnouncement
	for _, cat := range q.Categories {
		out = append(out, r.Announcement[cat]...)
	}
	return out, nil
}

// getAPI fetches a GET endpoint under the API base (cntSecurity).
func (c *Client) getAPI(ctx context.Context, endpoint string) ([]byte, error) {
	body, err := c.do(ctx, c.HTTP, maxAPIBody, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Referer", "https://www.cse.lk/")
		req.Header.Set("User-Agent", userAgent)
		return req, nil
	})
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", endpoint, err)
	}
	return body, nil
}
