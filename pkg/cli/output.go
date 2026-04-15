package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

// OutputFormat controls how results are displayed.
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
)

// PrintJSON pretty-prints data as JSON.
func PrintJSON(data any) {
	b, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(b))
}

// PrintRawJSON pretty-prints raw JSON bytes.
func PrintRawJSON(data []byte) {
	var v any
	if json.Unmarshal(data, &v) == nil {
		PrintJSON(v)
	} else {
		fmt.Println(string(data))
	}
}

// PrintTable prints a slice of maps as a table.
func PrintTable(items []map[string]any, columns []string) {
	if len(items) == 0 {
		fmt.Println("(no results)")
		return
	}

	// Auto-detect columns if not specified.
	if len(columns) == 0 {
		colSet := make(map[string]bool)
		for _, item := range items {
			for k := range item {
				colSet[k] = true
			}
		}
		for k := range colSet {
			columns = append(columns, k)
		}
		sort.Strings(columns)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header.
	headers := make([]string, len(columns))
	for i, c := range columns {
		headers[i] = strings.ToUpper(c)
	}
	fmt.Fprintln(w, strings.Join(headers, "\t"))

	// Rows.
	for _, item := range items {
		vals := make([]string, len(columns))
		for i, c := range columns {
			v := item[c]
			if v == nil {
				vals[i] = ""
			} else {
				vals[i] = fmt.Sprintf("%v", v)
			}
		}
		fmt.Fprintln(w, strings.Join(vals, "\t"))
	}
	w.Flush()
}

// PrintObject prints a single object as key-value pairs.
func PrintObject(obj map[string]any, keys []string) {
	if len(keys) == 0 {
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}
	maxLen := 0
	for _, k := range keys {
		if len(k) > maxLen {
			maxLen = len(k)
		}
	}
	for _, k := range keys {
		v := obj[k]
		if v == nil {
			continue
		}
		fmt.Printf("%-*s  %v\n", maxLen, k, v)
	}
}

// CheckError prints an error message from the API response and exits.
func CheckError(status int, data []byte) {
	if status >= 200 && status < 300 {
		return
	}
	var m map[string]any
	if json.Unmarshal(data, &m) == nil {
		if msg, ok := m["error"]; ok {
			fmt.Fprintf(os.Stderr, "Error (%d): %v\n", status, msg)
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stderr, "Error (%d): %s\n", status, string(data))
	os.Exit(1)
}
