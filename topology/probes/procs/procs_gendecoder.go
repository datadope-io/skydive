// Code generated - DO NOT EDIT.

package procs

import (
	"github.com/skydive-project/skydive/graffiti/getter"
	"strings"
)

func (obj *Endpoint) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *Endpoint) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "Port":
		return int64(obj.Port), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *Endpoint) GetFieldString(key string) (string, error) {
	switch key {
	case "Proc":
		return string(obj.Proc), nil
	case "IP":
		return string(obj.IP), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *Endpoint) GetFieldKeys() []string {
	return []string{
		"Proc",
		"IP",
		"Port",
	}
}

func (obj *Endpoint) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *Endpoint) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *Endpoint) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *Endpoint) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *Proc) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *Proc) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *Proc) GetFieldString(key string) (string, error) {
	switch key {
	case "Name":
		return string(obj.Name), nil
	case "SubType":
		return string(obj.SubType), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *Proc) GetFieldKeys() []string {
	return []string{
		"Name",
		"SubType",
		"TCPListen",
		"TCPConn",
	}
}

func (obj *Proc) MatchBool(key string, predicate getter.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "TCPListen":
		if index != -1 {
			for _, obj := range obj.TCPListen {
				if obj.MatchBool(key[index+1:], predicate) {
					return true
				}
			}
		}
	case "TCPConn":
		if index != -1 {
			for _, obj := range obj.TCPConn {
				if obj.MatchBool(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Proc) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "TCPListen":
		if index != -1 {
			for _, obj := range obj.TCPListen {
				if obj.MatchInt64(key[index+1:], predicate) {
					return true
				}
			}
		}
	case "TCPConn":
		if index != -1 {
			for _, obj := range obj.TCPConn {
				if obj.MatchInt64(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Proc) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "TCPListen":
		if index != -1 {
			for _, obj := range obj.TCPListen {
				if obj.MatchString(key[index+1:], predicate) {
					return true
				}
			}
		}
	case "TCPConn":
		if index != -1 {
			for _, obj := range obj.TCPConn {
				if obj.MatchString(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *Proc) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "TCPListen":
		if obj.TCPListen != nil {
			if index != -1 {
				var results []interface{}
				for _, obj := range obj.TCPListen {
					if field, err := obj.GetField(key[index+1:]); err == nil {
						results = append(results, field)
					}
				}
				return results, nil
			} else {
				var results []interface{}
				for _, obj := range obj.TCPListen {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "TCPConn":
		if obj.TCPConn != nil {
			if index != -1 {
				var results []interface{}
				for _, obj := range obj.TCPConn {
					if field, err := obj.GetField(key[index+1:]); err == nil {
						results = append(results, field)
					}
				}
				return results, nil
			} else {
				var results []interface{}
				for _, obj := range obj.TCPConn {
					results = append(results, obj)
				}
				return results, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func init() {
	strings.Index("", ".")
}
