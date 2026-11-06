package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"riha/internal/core/domain"
)

const callbackLogLimit = 4000

var phoneNumber = regexp.MustCompile(`\+?\b(?:258)?8[2-7]\d{7}\b`)

// railCallback takes whatever bim sends, in any method and format, because its format
// isn't known yet. It logs it and always says OK. See App.RailCallback for what it does.
func (srv *Server) railCallback(c *gin.Context) {
	raw, _ := io.ReadAll(io.LimitReader(c.Request.Body, 64<<10))
	names := make([]string, 0, len(c.Request.Header))
	for k := range c.Request.Header {
		names = append(names, k)
	}
	sort.Strings(names)
	// Only the names of the headers: one of them may hold a signature or a key.
	log.Printf("bim callback: %s from %s type=%q agent=%q headers=%v query=%q body=%q", c.Request.Method, c.ClientIP(), c.ContentType(), c.GetHeader("User-Agent"), names,
		clean(c.Request.URL.RawQuery), clean(string(raw)))
	in := domain.RequestFrom(ctx(c))
	if op, id := callbackOperation(raw, c.Request.URL.Query()); op != "" {
		go srv.A.RailCallback(domain.WithRequest(context.Background(), in), op, id, raw)
	} else {
		log.Printf("bim callback: no operation number found")
		srv.A.TraceCallbackWithoutOperation(ctx(c), c.Request.Method, raw)
	}
	c.JSON(200, gin.H{"ok": true})
}

func clean(s string) string {
	s = phoneNumber.ReplaceAllString(s, "<number>")
	if len(s) > callbackLogLimit {
		s = s[:callbackLogLimit] + "…"
	}
	return s
}

var railIDShape = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

// callbackOperation finds the operation number and the transaction id in a JSON body,
// a form body or the query.
func callbackOperation(raw []byte, query url.Values) (string, string) {
	var v any
	if json.Unmarshal(raw, &v) == nil {
		if op := stringIn(v, "operationNumber"); op != "" {
			return op, checkedID(stringIn(v, "transactionID_NEP"))
		}
	}
	if form, err := url.ParseQuery(string(raw)); err == nil {
		if op := valueIn(form, "operationNumber"); op != "" {
			return op, checkedID(valueIn(form, "transactionID_NEP"))
		}
	}
	return valueIn(query, "operationNumber"), checkedID(valueIn(query, "transactionID_NEP"))
}

func checkedID(id string) string {
	if railIDShape.MatchString(id) {
		return id
	}
	return ""
}

func stringIn(v any, name string) string {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			if strings.EqualFold(k, name) {
				if s, ok := x.(string); ok && s != "" {
					return s
				}
			}
		}
		for _, x := range t {
			if s := stringIn(x, name); s != "" {
				return s
			}
		}
	case []any:
		for _, x := range t {
			if s := stringIn(x, name); s != "" {
				return s
			}
		}
	}
	return ""
}

func valueIn(values url.Values, name string) string {
	for k, x := range values {
		if strings.EqualFold(k, name) && len(x) > 0 {
			return x[0]
		}
	}
	return ""
}
