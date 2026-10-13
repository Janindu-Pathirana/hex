package engine

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/Janindu-Pathirana/hex/internal/cse"
	"github.com/Janindu-Pathirana/hex/internal/store"
)

// RecordRaw makes c save every API response body into db's raw layer, keyed by
// endpoint and form. Save failures are logged, not fatal.
func RecordRaw(c *cse.Client, db *store.DB, log io.Writer) {
	c.Observe = func(endpoint string, form url.Values, body []byte) {
		ctx := context.Background()
		err := db.Update(ctx, func(tx *store.Tx) error {
			return tx.SaveRaw(ctx, "cse", endpoint, form.Encode(), body)
		})
		if err != nil {
			fmt.Fprintf(log, "warning: %v\n", err)
		}
	}
}
