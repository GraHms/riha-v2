package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"riha/internal/adapters/rails/mtoca"
	"riha/internal/core/domain"
)

func main() {
	base := flag.String("url", "https://api.payments.dev.mtoca.net", "API base URL")
	phone := flag.String("phone", "", "payer number, for example 258841234567")
	amount := flag.String("amount", "10.00", "amount in MT, for example 10.00")
	channel := flag.String("channel", "BIM", "mtoca channel")
	desc := flag.String("desc", "RIHA teste de integracao", "description")
	op := flag.String("op", "", "operation number (default: a new random one)")
	token := flag.String("token", os.Getenv("RIHA_MTOCA_TOKEN"), "bearer token, if the API needs one")
	statusOnly := flag.Bool("status-only", false, "only check the status of -op and -nep")
	nep := flag.String("nep", "", "transaction id from the create call, for -status-only")
	every := flag.Duration("every", 3*time.Second, "status polling interval")
	wait := flag.Duration("for", 3*time.Minute, "how long to poll")
	prd := flag.Bool("i-know-this-is-production", false, "allow the production URL")
	flag.Parse()

	if strings.Contains(*base, "api.payments.mtoca.net") && !strings.Contains(*base, ".dev.") && !*prd {
		fail("refusing to call production without -i-know-this-is-production")
	}
	digits := domain.NormalizeMZPhone(*phone)
	if len(digits) != 9 && !*statusOnly {
		fail("use -phone with a Mozambican number, for example 258841234567")
	}
	if *op == "" {
		if *statusOnly {
			fail("-status-only needs -op")
		}
		*op = domain.OperationNumber()
	}
	amt := json.RawMessage(strings.Replace(*amount, ",", ".", 1))
	id := *nep
	fmt.Printf("operationNumber: %s\n\n", *op)
	if !*statusOnly {
		m, final := call(*base+"/api/payments", map[string]any{"operationNumber": *op, "channel": *channel, "amount": amt, "phoneNumber": "258" + digits, "description": *desc}, *token)
		if final {
			return
		}
		id, _ = m["vendorTransactionId_NEP"].(string)
		fmt.Printf("vendorTransactionId_NEP: %s\n\n", id)
	}
	if id == "" {
		fail("no transaction id, so the status can't be asked. For -status-only pass -nep")
	}
	status := map[string]any{"operationNumber": *op, "transactionID_NEP": id}
	deadline := time.Now().Add(*wait)
	for {
		if _, final := call(*base+"/api/payments/status", status, *token); final || *statusOnly || time.Now().After(deadline) {
			return
		}
		time.Sleep(*every)
	}
}

func call(url string, b map[string]any, token string) (map[string]any, bool) {
	raw, _ := json.Marshal(b)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	fmt.Printf("%s  POST %s\n  → %s\n", time.Now().Format("15:04:05"), url, raw)
	res, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		fmt.Printf("  ✗ %v\n\n", err)
		return nil, false
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	fmt.Printf("  ← HTTP %d %s\n  %s\n", res.StatusCode, res.Header.Get("Content-Type"), out)
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if res.StatusCode >= 300 {
		fmt.Println()
		return m, strings.HasSuffix(url, "/api/payments")
	}
	o, final := mtoca.Interpret(m)
	switch {
	case !final:
		fmt.Printf("  RIHA reads this as: still pending\n\n")
	case o.Success:
		fmt.Printf("  RIHA reads this as: PAID (operator reference %q)\n\n", o.OperatorReference)
	default:
		fmt.Printf("  RIHA reads this as: FAILED, %s: %s\n\n", o.FailureCode, domain.FailureMessages[o.FailureCode])
	}
	return m, final
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}
