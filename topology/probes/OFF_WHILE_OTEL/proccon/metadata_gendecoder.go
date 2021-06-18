// Code generated - DO NOT EDIT.

package proccon

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *ProcInfo) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *ProcInfo) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "CreatedAt":
		return int64(obj.CreatedAt), nil
	case "UpdatedAt":
		return int64(obj.UpdatedAt), nil
	case "Revision":
		return int64(obj.Revision), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *ProcInfo) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *ProcInfo) GetFieldKeys() []string {
	return []string{
		"CreatedAt",
		"UpdatedAt",
		"Revision",
	}
}

func (obj *ProcInfo) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *ProcInfo) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *ProcInfo) MatchString(key string, predicate getter.StringPredicate) bool {
	return false
}

func (obj *ProcInfo) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *NetworkInfo) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *NetworkInfo) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *NetworkInfo) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *NetworkInfo) GetFieldKeys() []string {
	return []string{}
}

func (obj *NetworkInfo) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *NetworkInfo) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *NetworkInfo) MatchString(key string, predicate getter.StringPredicate) bool {
	return false
}

func (obj *NetworkInfo) GetField(key string) (interface{}, error) {
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
