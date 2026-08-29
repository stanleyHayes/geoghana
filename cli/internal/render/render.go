// Package render writes results as a human table, JSON or CSV.
//
// The CLI is meant to be scriptable as readily as readable: --json for jq,
// --csv for a spreadsheet, and a table by default. Colour is suppressed when
// output is piped, and honours NO_COLOR (https://no-color.org).
package render

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatCSV   Format = "csv"
)

// Colour is disabled unless the destination is a terminal and NO_COLOR is
// unset, so piping never injects escape codes into a script's input.
func ColourEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

const (
	reset = "\x1b[0m"
	dim   = "\x1b[2m"
	bold  = "\x1b[1m"
)

type Table struct {
	Headers []string
	Rows    [][]string
	// Note is printed under the table, e.g. the dataset version.
	Note string
}

// WriteTable prints an aligned table. Width is measured in runes, so Ghanaian
// orthography (ɔ, ɛ, ŋ) does not break column alignment the way byte length would.
func WriteTable(w io.Writer, t Table) error {
	colour := ColourEnabled(w)
	widths := make([]int, len(t.Headers))
	for i, h := range t.Headers {
		widths[i] = utf8.RuneCountInString(h)
	}
	for _, row := range t.Rows {
		for i, cell := range row {
			if i < len(widths) {
				if n := utf8.RuneCountInString(cell); n > widths[i] {
					widths[i] = n
				}
			}
		}
	}

	var b strings.Builder
	for i, h := range t.Headers {
		if colour {
			b.WriteString(bold)
		}
		b.WriteString(pad(strings.ToUpper(h), widths[i]))
		if colour {
			b.WriteString(reset)
		}
		if i < len(t.Headers)-1 {
			b.WriteString("  ")
		}
	}
	b.WriteString("\n")

	for _, row := range t.Rows {
		for i := range t.Headers {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			b.WriteString(pad(cell, widths[i]))
			if i < len(t.Headers)-1 {
				b.WriteString("  ")
			}
		}
		b.WriteString("\n")
	}
	if t.Note != "" {
		if colour {
			b.WriteString(dim)
		}
		b.WriteString("\n" + t.Note)
		if colour {
			b.WriteString(reset)
		}
		b.WriteString("\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func pad(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func WriteCSV(w io.Writer, t Table) error {
	c := csv.NewWriter(w)
	if err := c.Write(t.Headers); err != nil {
		return err
	}
	for _, row := range t.Rows {
		if err := c.Write(row); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}

// Warn writes to stderr so it never contaminates piped output.
func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Coord renders a coordinate, or a dash when the dataset has none yet.
func Coord(lat, lng *float64) string {
	if lat == nil || lng == nil {
		return "—"
	}
	return fmt.Sprintf("%.4f, %.4f", *lat, *lng)
}

// Status abbreviates a verification status for a narrow column, keeping a
// non-colour carrier so meaning survives a monochrome terminal.
func Status(s string) string {
	switch s {
	case "CANONICAL":
		return "✓ canonical"
	case "REVIEWED":
		return "◆ reviewed"
	case "SEED_NEEDS_CANONICAL_RECONCILIATION":
		return "! needs recon"
	case "REFERENCE":
		return "· reference"
	case "":
		return ""
	default:
		return strings.ToLower(s)
	}
}
