package postgres

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"riha/internal/core/domain"
)

type StringList []string

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(s))
	return string(b), err
}

func (s *StringList) Scan(v any) error { return scanJSON(v, (*[]string)(s)) }

type StringMap map[string]string

func (m StringMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(map[string]string(m))
	return string(b), err
}

func (m *StringMap) Scan(v any) error {
	out := map[string]string{}
	if err := scanJSON(v, &out); err != nil {
		return err
	}
	*m = out
	return nil
}

type LineItems []domain.LineItem

func (l LineItems) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]domain.LineItem(l))
	return string(b), err
}

func (l *LineItems) Scan(v any) error { return scanJSON(v, (*[]domain.LineItem)(l)) }

func scanJSON(v any, dst any) error {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		return json.Unmarshal([]byte(t), dst)
	case []byte:
		return json.Unmarshal(t, dst)
	}
	return errors.New("unsupported json column")
}

var timeType = reflect.TypeOf(time.Time{})

func copyInto(dst, src any) {
	copyValue(reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem())
}

func copyValue(dst, src reflect.Value) {
	for i := 0; i < dst.NumField(); i++ {
		f := dst.Type().Field(i)
		if !f.IsExported() {
			continue
		}
		sv := src.FieldByName(f.Name)
		if !sv.IsValid() {
			continue
		}
		dv := dst.Field(i)
		switch {
		case sv.Type().AssignableTo(dv.Type()):
			dv.Set(sv)
		case sv.Type().ConvertibleTo(dv.Type()):
			dv.Set(sv.Convert(dv.Type()))
		case sv.Kind() == reflect.Struct && dv.Kind() == reflect.Struct && dv.Type() != timeType:
			copyValue(dv, sv)
		}
	}
}

func toDomain[D any, R any](r *R) *D {
	var d D
	copyInto(&d, r)
	return &d
}

func toRecord[R any, D any](d *D) *R {
	var r R
	copyInto(&r, d)
	return &r
}

func many[D any, R any](rs []R) []D {
	out := make([]D, len(rs))
	for i := range rs {
		copyInto(&out[i], &rs[i])
	}
	return out
}
