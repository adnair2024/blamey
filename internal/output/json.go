// Package output renders a stats.Report for non-interactive use.
package output

import (
	"encoding/json"
	"io"

	"github.com/adnair2024/blamey/internal/stats"
)

// JSON writes the full report as indented JSON.
func JSON(w io.Writer, r stats.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
