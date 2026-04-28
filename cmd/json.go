package cmd

import (
	"encoding/json"
	"os"
	"time"

	"github.com/nickhudkins/tokens/ccusage"
)

type jsonEnvelope struct {
	FetchedAt time.Time          `json:"fetched_at"`
	FromCache bool               `json:"from_cache"`
	Data      *ccusage.UsageData `json:"data"`
}

func emitJSON(res *ccusage.FetchResult) error {
	out := jsonEnvelope{
		FetchedAt: res.FetchedAt,
		FromCache: res.FromCache,
		Data:      res.Data,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
