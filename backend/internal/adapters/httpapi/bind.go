package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"riha/internal/core/domain"
)

var bracket = regexp.MustCompile(`\[([^\]]*)\]`)
var numeric = regexp.MustCompile(`^-?[0-9]+$`)

var stringKeys = map[string]bool{"client_reference_id": true, "description": true, "name": true, "phone": true, "email": true, "slug": true, "url": true, "reason": true}
var listKeys = map[string]bool{"payment_method_types": true, "enabled_events": true}

func bind(c *gin.Context, dst any) error {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	ct := c.ContentType()
	if ct == "application/x-www-form-urlencoded" || (ct == "" && !strings.HasPrefix(strings.TrimSpace(string(raw)), "{")) {
		vals, err := url.ParseQuery(string(raw))
		if err != nil {
			return domain.Invalid("body_invalid", "", "Corpo do pedido inválido.")
		}
		tree := map[string]any{}
		for k, vs := range vals {
			setPath(tree, k, vs)
		}
		raw, _ = json.Marshal(arrays(tree))
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		var te *json.UnmarshalTypeError
		if errors.As(err, &te) {
			return domain.Invalid("parameter_invalid", te.Field, "Tipo inválido para "+te.Field+".")
		}
		return domain.Invalid("body_invalid", "", "JSON inválido: "+err.Error())
	}
	return nil
}

func scalar(key string, isMeta bool, v string) any {
	if isMeta {
		return v
	}
	if !stringKeys[key] && numeric.MatchString(v) {
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	if v == "true" || v == "false" {
		return v == "true"
	}
	return v
}

func setPath(tree map[string]any, key string, vs []string) {
	base, path := key, []string{key}
	if i := strings.Index(key, "["); i > 0 {
		base = key[:i]
		path = []string{base}
		for _, m := range bracket.FindAllStringSubmatch(key[i:], -1) {
			path = append(path, m[1])
		}
	}
	if listKeys[base] {
		lst := []any{}
		for _, x := range vs {
			for _, y := range strings.Split(x, ",") {
				lst = append(lst, strings.TrimSpace(y))
			}
		}
		if prev, ok := tree[base].([]any); ok {
			lst = append(prev, lst...)
		}
		tree[base] = lst
		return
	}
	cur := tree
	for i, p := range path {
		if i == len(path)-1 {
			cur[p] = scalar(p, path[0] == "metadata" && len(path) > 1, vs[len(vs)-1])
			return
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
}

func arrays(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	indexed := len(m) > 0
	for k := range m {
		if _, err := strconv.Atoi(k); err != nil {
			indexed = false
		}
		m[k] = arrays(m[k])
	}
	if !indexed {
		return m
	}
	out := make([]any, len(m))
	for k, x := range m {
		if i, _ := strconv.Atoi(k); i < len(out) {
			out[i] = x
		}
	}
	return out
}
