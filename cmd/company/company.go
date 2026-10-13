// Package company holds the logic behind "hex company".
package company

import (
	"context"
	"fmt"

	"github.com/Janindu-Pathirana/hex/internal"
	"github.com/Janindu-Pathirana/hex/internal/cache"
	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/Janindu-Pathirana/hex/internal/output"
)

// Securities returns the listed securities, from the cache when it is fresh.
func Securities(ctx context.Context, c *cse.Client) ([]cse.Security, error) {
	var secs []cse.Security
	ok, err := cache.Load(internal.SecuritiesCache, &secs)
	if err != nil {
		return nil, err
	}
	if ok && len(secs) > 0 {
		return secs, nil
	}

	secs, err = c.Securities(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch securities: %w", err)
	}
	if err := cache.Save(internal.SecuritiesCache, secs); err != nil {
		return nil, fmt.Errorf("save securities: %w", err)
	}
	return secs, nil
}

// Resolve maps user input ("jkh", "CIC.X0000") to a listed security.
func Resolve(ctx context.Context, c *cse.Client, input string) (cse.Security, error) {
	secs, err := Securities(ctx, c)
	if err != nil {
		return cse.Security{}, err
	}
	return cse.Resolve(input, secs)
}

// ReportView is a report PDF in a readable shape.
type ReportView struct {
	Title      string  `json:"title"`
	Date       string  `json:"date"`
	Uploaded   string  `json:"uploaded"`
	Authorized *string `json:"authorized"`
	URL        string  `json:"url"`
}

// ReportViews converts API reports into ReportViews, keeping their order.
func ReportViews(reports []cse.Report) []ReportView {
	views := make([]ReportView, len(reports))
	for i, r := range reports {
		v := ReportView{
			Title:    r.FileText,
			Date:     output.Date(r.ManualDate),
			Uploaded: output.DateTime(r.UploadedDate),
			URL:      cse.AssetURL(r.Path),
		}
		if r.AuthorizedDate != nil {
			a := output.DateTime(*r.AuthorizedDate)
			v.Authorized = &a
		}
		views[i] = v
	}
	return views
}
