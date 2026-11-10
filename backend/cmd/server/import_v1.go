package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"riha/internal/core/app"
)

// importV1 runs `riha import-v1 -file accounts.json [-apply]`: it brings the
// first RIHA's accounts over, or with no -apply says what it would do.
func importV1(ctx context.Context, a *app.App, args []string) int {
	fs := flag.NewFlagSet("import-v1", flag.ContinueOnError)
	file := fs.String("file", "", "JSON list of v1 accounts")
	apply := fs.Bool("apply", false, "write to the database (without it, only report)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "import-v1: -file is required")
		return 2
	}
	raw, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "import-v1:", err)
		return 1
	}
	var accounts []app.V1Account
	if err := json.Unmarshal(raw, &accounts); err != nil {
		fmt.Fprintln(os.Stderr, "import-v1: reading the file:", err)
		return 1
	}
	out, err := a.ImportV1(ctx, accounts, *apply)
	reasons := map[string]int{}
	imported := 0
	for _, o := range out {
		if o.Imported {
			imported++
			continue
		}
		reasons[o.Reason]++
		fmt.Printf("skip  %-40.40s %s\n", o.Business, o.Reason)
	}
	keys := make([]string, 0, len(reasons))
	for k := range reasons {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	verb := "would import"
	if *apply {
		verb = "imported"
	}
	fmt.Printf("\n%d accounts in the file, %s %d\n", len(accounts), verb, imported)
	for _, k := range keys {
		fmt.Printf("  skipped %3d  %s\n", reasons[k], k)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "import-v1: stopped after %d accounts: %v\n", len(out), err)
		return 1
	}
	if !*apply {
		fmt.Println("\nNothing was written. Run again with -apply to import.")
	}
	return 0
}
