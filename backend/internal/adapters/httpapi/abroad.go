package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

// abroadCallback takes U2opia's word on a payment from abroad, sent as JSON or
// as query parameters, whichever U2opia is set up for. It always answers 200:
// U2opia can't act on a refusal, and the trail says what was done with it.
func (srv *Server) abroadCallback(c *gin.Context) {
	raw, _ := io.ReadAll(io.LimitReader(c.Request.Body, 64<<10))
	r := readAbroadResult(raw, c.Request.URL.Query())
	if r.TransactionID == "" {
		log.Printf("u2opia callback: no transaction id, query=%q body=%q", clean(c.Request.URL.RawQuery), clean(string(raw)))
		c.JSON(200, gin.H{"ok": true})
		return
	}
	in := domain.RequestFrom(ctx(c))
	domain.Safely("u2opia callback", func() { srv.A.AbroadCallback(domain.WithRequest(context.Background(), in), r) })
	c.JSON(200, gin.H{"ok": true})
}

func readAbroadResult(raw []byte, q url.Values) app.AbroadResult {
	var b map[string]any
	_ = json.Unmarshal(raw, &b)
	get := func(k string) string {
		if v, ok := b[k]; ok && v != nil {
			switch x := v.(type) {
			case string:
				return strings.TrimSpace(x)
			case float64:
				return strconv.FormatFloat(x, 'f', -1, 64)
			}
		}
		return strings.TrimSpace(q.Get(k))
	}
	status, _ := strconv.Atoi(get("transactionStatus"))
	return app.AbroadResult{TransactionID: get("transactionId"), Status: status, Reference: get("referenceNumber"), Amount: get("totalAmount"),
		Channel: get("paymentChannel"), Raw: raw}
}

// abroadReturn sends a customer back from U2opia's page to their checkout.
// U2opia's names for the transaction vary, so the usual ones are all read.
func (srv *Server) abroadReturn(c *gin.Context) {
	q := c.Request.URL.Query()
	id := domain.FirstNonEmpty(q.Get("transactionId"), q.Get("transaction_id"), q.Get("txnId"), q.Get("transactionID"))
	if id == "" {
		log.Printf("u2opia return: no transaction id, query=%q", clean(c.Request.URL.RawQuery))
	}
	c.Redirect(302, srv.A.AbroadReturn(ctx(c), id, domain.FirstNonEmpty(q.Get("status"), q.Get("transactionStatus"))))
}
