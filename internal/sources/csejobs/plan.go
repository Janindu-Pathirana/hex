package csejobs

import (
	"time"

	"github.com/Janindu-Pathirana/hex/internal/engine"
)

// Scope selects which data groups a sync covers.
type Scope struct {
	Securities bool
	Market     bool
	Quotes     bool
	Info       bool
	History    bool
	Profiles   bool
	Reports    bool
	Actions    bool
}

// Plan builds the job stages for scope and the given security codes.
// Stage 0 holds the market-wide bulk calls, stage 1 per-company detail
// (which stores the stockId), and stage 2 history (which needs it).
func Plan(scope Scope, codes []string, now time.Time) []engine.Stage {
	live := 12 * time.Hour
	quotes := 12 * time.Hour
	if MarketOpen(now) {
		live, quotes = 15*time.Minute, 5*time.Minute
	}

	var bulk, detail, history engine.Stage
	if scope.Securities {
		bulk = append(bulk, Securities())
	}
	if scope.Market {
		bulk = append(bulk, Market(live))
	}
	if scope.Quotes {
		bulk = append(bulk, Quotes(quotes))
	}
	if scope.Actions {
		detail = append(detail, Actions())
	}
	for _, code := range codes {
		if scope.Info || scope.History {
			detail = append(detail, Info(code))
		}
		if scope.Profiles {
			detail = append(detail, Profile(code))
		}
		if scope.Reports {
			detail = append(detail, Reports(code))
		}
		if scope.History {
			history = append(history, History(code))
		}
	}

	var stages []engine.Stage
	for _, st := range []engine.Stage{bulk, detail, history} {
		if len(st) > 0 {
			stages = append(stages, st)
		}
	}
	return stages
}
