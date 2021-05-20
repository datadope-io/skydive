// Code generated - DO NOT EDIT.

package netexternal

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *Route) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *Route) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *Route) GetFieldString(key string) (string, error) {
	switch key {
	case "Name":
		return string(obj.Name), nil
	case "Network":
		return obj.Network.String(), nil
	case "NextHop":
		return obj.NextHop.String(), nil
	case "DeviceNextHop":
		return string(obj.DeviceNextHop), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *Route) GetFieldKeys() []string {
	return []string{
		"Name",
		"Network",
		"NextHop",
		"DeviceNextHop",
	}
}

func (obj *Route) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *Route) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *Route) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *Route) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *RouteTable) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *RouteTable) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *RouteTable) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *RouteTable) GetFieldKeys() []string {
	return []string{}
}

func (obj *RouteTable) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *RouteTable) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *RouteTable) MatchString(key string, predicate getter.StringPredicate) bool {
	return false
}

func (obj *RouteTable) GetField(key string) (interface{}, error) {
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
