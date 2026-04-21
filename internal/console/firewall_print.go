package console

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

func stringifyJSMatrix(raw interface{}) [][]string {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rowValues, ok := item.([]interface{})
		if !ok {
			if text, ok := item.(string); ok {
				text = strings.TrimSpace(text)
				if text != "" {
					rows = append(rows, []string{text})
				}
			}
			continue
		}

		row := make([]string, 0, len(rowValues))
		for _, value := range rowValues {
			text, ok := value.(string)
			if !ok {
				continue
			}
			row = append(row, strings.TrimSpace(text))
		}

		if len(row) > 0 {
			rows = append(rows, row)
		}
	}

	return rows
}

func printFirewallTabRows(tabLabel, name, idx string, rows [][]string) {
	fmt.Printf("\n=== [ELCAP %s 정책 | %s | IDX: %s] ===\n", tabLabel, strings.TrimSpace(name), idx)

	maxCols := 0
	for _, row := range rows {
		if len(row) > maxCols {
			maxCols = len(row)
		}
	}

	if maxCols <= 1 {
		for i, row := range rows {
			if len(row) == 0 {
				continue
			}
			fmt.Printf("[%d] %s\n", i+1, strings.TrimSpace(row[0]))
		}
		fmt.Println("========================================")
		return
	}

	normalized := normalizeRows(rows, maxCols)
	header := normalized[0]
	data := normalized[1:]

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintln(writer, strings.Join(header, "\t"))
	fmt.Fprintln(writer, strings.Join(makeDivider(header), "\t"))
	for _, row := range data {
		fmt.Fprintln(writer, strings.Join(row, "\t"))
	}
	_ = writer.Flush()

	fmt.Printf("총 %d건\n", len(data))
	fmt.Println("========================================")
}

func normalizeRows(rows [][]string, maxCols int) [][]string {
	result := make([][]string, 0, len(rows))
	for _, row := range rows {
		normalized := make([]string, maxCols)
		copy(normalized, row)
		result = append(result, normalized)
	}
	return result
}

func makeDivider(header []string) []string {
	divider := make([]string, len(header))
	for i := range header {
		divider[i] = "----"
	}
	return divider
}
