// Code generated - DO NOT EDIT.

package graphql

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *Event) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *Event) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "TTL":
		return int64(obj.TTL), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *Event) GetFieldString(key string) (string, error) {
	switch key {
	case "Level":
		return string(obj.Level), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *Event) GetFieldKeys() []string {
	return []string{
		"TTL",
		"Level",
		"Payload",
		"CreatedAt",
		"UpdatedAt",
	}
}

func (obj *Event) MatchBool(key string, predicate getter.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Payload":
		if index != -1 {
			for _, obj := range obj.Payload {
				if obj.MatchBool(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Event) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Payload":
		if index != -1 {
			for _, obj := range obj.Payload {
				if obj.MatchInt64(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Event) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Payload":
		if index != -1 {
			for _, obj := range obj.Payload {
				if obj.MatchString(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Event) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Payload":
		if obj.Payload != nil {
			if index != -1 {
				var results []interface{}
				for _, obj := range obj.Payload {
					if field, err := obj.GetField(key[index+1:]); err == nil {
						results = append(results, field)
					}
				}
				return results, nil
			} else {
				var results []interface{}
				for _, obj := range obj.Payload {
					results = append(results, obj)
				}
				return results, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func (obj *Events) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *Events) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *Events) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *Events) GetFieldKeys() []string {
	return []string{}
}

func (obj *Events) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *Events) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *Events) MatchString(key string, predicate getter.StringPredicate) bool {
	return false
}

func (obj *Events) GetField(key string) (interface{}, error) {
	return nil, getter.ErrFieldNotFound
}

func (obj *PayloadValues) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *PayloadValues) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *PayloadValues) GetFieldString(key string) (string, error) {
	switch key {
	case "Key":
		return string(obj.Key), nil
	case "Value":
		return string(obj.Value), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *PayloadValues) GetFieldKeys() []string {
	return []string{
		"Key",
		"Value",
	}
}

func (obj *PayloadValues) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *PayloadValues) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *PayloadValues) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *PayloadValues) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
