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
	if op := callbackOperation(raw, c.Request.URL.Query()); op != "" {
		go srv.A.RailCallback(context.Background(), op)
	} else {
		log.Printf("bim callback: no operation number found")
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

// callbackOperation looks for the operation number in a JSON body, a form body or the query.
func callbackOperation(raw []byte, query url.Values) string {
	var v any
	if json.Unmarshal(raw, &v) == nil {
		if op := operationIn(v); op != "" {
			return op
		}
	}
	if form, err := url.ParseQuery(string(raw)); err == nil {
		if op := operationInValues(form); op != "" {
			return op
		}
	}
	return operationInValues(query)
}

func operationIn(v any) string {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			if strings.EqualFold(k, "operationNumber") {
				if s, ok := x.(string); ok && s != "" {
					return s
				}
			}
		}
		for _, x := range t {
			if op := operationIn(x); op != "" {
				return op
			}
		}
	case []any:
		for _, x := range t {
			if op := operationIn(x); op != "" {
				return op
			}
		}
	}
	return ""
}

func operationInValues(values url.Values) string {
	for k, x := range values {
		if strings.EqualFold(k, "operationNumber") && len(x) > 0 {
			return x[0]
		}
	}
	return ""
}
