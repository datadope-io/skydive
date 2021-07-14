// Code generated - DO NOT EDIT.

package netexternal

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *AlarmsML) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *AlarmsML) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *AlarmsML) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *AlarmsML) GetFieldKeys() []string {
	return []string{}
}

func (obj *AlarmsML) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *AlarmsML) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *AlarmsML) MatchString(key string, predicate getter.StringPredicate) bool {
	return false
}

func (obj *AlarmsML) GetField(key string) (interface{}, error) {
	return nil, getter.ErrFieldNotFound
}

func (obj *AlarmsMLEvent) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *AlarmsMLEvent) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Span":
		return int64(obj.Span), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *AlarmsMLEvent) GetFieldString(key string) (string, error) {
	switch key {
	case "Id":
		return string(obj.Id), nil
	case "Function":
		return string(obj.Function), nil
	case "Score":
		return string(obj.Score), nil
	case "Probability":
		return string(obj.Probability), nil
	case "Field":
		return string(obj.Field), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *AlarmsMLEvent) GetFieldKeys() []string {
	return []string{
		"Id",
		"Span",
		"Function",
		"Score",
		"Probability",
		"Field",
		"CreatedAt",
		"UpdatedAt",
		"DeletedAt",
	}
}

func (obj *AlarmsMLEvent) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *AlarmsMLEvent) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *AlarmsMLEvent) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *AlarmsMLEvent) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
