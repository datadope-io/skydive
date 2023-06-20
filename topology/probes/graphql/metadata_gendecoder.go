// Code generated - DO NOT EDIT.

package graphql

import (
	"strings"

	"github.com/skydive-project/skydive/graffiti/getter"
)

func (obj *EthernetInfo) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *EthernetInfo) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *EthernetInfo) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *EthernetInfo) GetFieldKeys() []string {
	return []string{
		"VLAN",
	}
}

func (obj *EthernetInfo) MatchBool(key string, predicate getter.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "VLAN":
		if index != -1 && obj.VLAN != nil {
			return obj.VLAN.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *EthernetInfo) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "VLAN":
		if index != -1 && obj.VLAN != nil {
			return obj.VLAN.MatchInt64(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *EthernetInfo) MatchString(key string, predicate getter.StringPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "VLAN":
		if index != -1 && obj.VLAN != nil {
			return obj.VLAN.MatchString(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *EthernetInfo) GetField(key string) (interface{}, error) {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "VLAN":
		if obj.VLAN != nil {
			if index != -1 {
				return obj.VLAN.GetField(key[index+1:])
			} else {
				return obj.VLAN, nil
			}
		}
	}
	return nil, getter.ErrFieldNotFound
}

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
	case "ChildID":
		return string(obj.ChildID), nil
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
		"DeletedAt",
		"ChildChain",
		"ChildID",
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
	case "ChildChain":
		if index == -1 {
			for _, i := range obj.ChildChain {
				if predicate(i) {
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
	case "ChildChain":
		if obj.ChildChain != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.ChildChain {
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
	return (*obj)[key], nil
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
	return []string{
		"Ethernet",
		"IP",
		"PseudoWires",
	}
}

func (obj *NetworkInfo) MatchBool(key string, predicate getter.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Ethernet":
		if index != -1 && obj.Ethernet != nil {
			return obj.Ethernet.MatchBool(key[index+1:], predicate)
		}
	case "PseudoWires":
		if index != -1 {
			for _, obj := range obj.PseudoWires {
				if obj.MatchBool(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *NetworkInfo) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Ethernet":
		if index != -1 && obj.Ethernet != nil {
			return obj.Ethernet.MatchInt64(key[index+1:], predicate)
		}
	case "PseudoWires":
		if index != -1 {
			for _, obj := range obj.PseudoWires {
				if obj.MatchInt64(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *NetworkInfo) MatchString(key string, predicate getter.StringPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Ethernet":
		if index != -1 && obj.Ethernet != nil {
			return obj.Ethernet.MatchString(key[index+1:], predicate)
		}
	case "IP":
		if index == -1 {
			for _, i := range obj.IP {
				if predicate(i) {
					return true
				}
			}
		}
	case "PseudoWires":
		if index != -1 {
			for _, obj := range obj.PseudoWires {
				if obj.MatchString(key[index+1:], predicate) {
					return true
				}
			}
		}
	}
	return false
}

func (obj *NetworkInfo) GetField(key string) (interface{}, error) {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Ethernet":
		if obj.Ethernet != nil {
			if index != -1 {
				return obj.Ethernet.GetField(key[index+1:])
			} else {
				return obj.Ethernet, nil
			}
		}
	case "IP":
		if obj.IP != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.IP {
					results = append(results, obj)
				}
				return results, nil
			}
		}
	case "PseudoWires":
		if obj.PseudoWires != nil {
			if index != -1 {
				var results []interface{}
				for _, obj := range obj.PseudoWires {
					if field, err := obj.GetField(key[index+1:]); err == nil {
						results = append(results, field)
					}
				}
				return results, nil
			} else {
				var results []interface{}
				for _, obj := range obj.PseudoWires {
					results = append(results, obj)
				}
				return results, nil
			}
		}

	}
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

func (obj *PseudoWireInfo) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *PseudoWireInfo) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *PseudoWireInfo) GetFieldString(key string) (string, error) {
	switch key {
	case "ID":
		return string(obj.ID), nil
	case "Neighbor":
		return string(obj.Neighbor), nil
	case "LocalVpws":
		return string(obj.LocalVpws), nil
	case "RemoteVpws":
		return string(obj.RemoteVpws), nil
	}
	return "", getter.ErrFieldNotFound
}

func (obj *PseudoWireInfo) GetFieldKeys() []string {
	return []string{
		"ID",
		"Neighbor",
		"LocalVpws",
		"RemoteVpws",
	}
}

func (obj *PseudoWireInfo) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *PseudoWireInfo) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	return false
}

func (obj *PseudoWireInfo) MatchString(key string, predicate getter.StringPredicate) bool {
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *PseudoWireInfo) GetField(key string) (interface{}, error) {
	if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *VLANAccessInfo) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *VLANAccessInfo) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "VID":
		return int64(obj.VID), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *VLANAccessInfo) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *VLANAccessInfo) GetFieldKeys() []string {
	return []string{
		"VID",
	}
}

func (obj *VLANAccessInfo) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *VLANAccessInfo) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *VLANAccessInfo) MatchString(key string, predicate getter.StringPredicate) bool {
	return false
}

func (obj *VLANAccessInfo) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	return nil, getter.ErrFieldNotFound
}

func (obj *VLANInfo) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *VLANInfo) GetFieldInt64(key string) (int64, error) {
	return 0, getter.ErrFieldNotFound
}

func (obj *VLANInfo) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *VLANInfo) GetFieldKeys() []string {
	return []string{
		"Access",
		"Trunk",
	}
}

func (obj *VLANInfo) MatchBool(key string, predicate getter.BoolPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Access":
		if index != -1 && obj.Access != nil {
			return obj.Access.MatchBool(key[index+1:], predicate)
		}
	case "Trunk":
		if index != -1 && obj.Trunk != nil {
			return obj.Trunk.MatchBool(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *VLANInfo) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Access":
		if index != -1 && obj.Access != nil {
			return obj.Access.MatchInt64(key[index+1:], predicate)
		}
	case "Trunk":
		if index != -1 && obj.Trunk != nil {
			return obj.Trunk.MatchInt64(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *VLANInfo) MatchString(key string, predicate getter.StringPredicate) bool {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {

	case "Access":
		if index != -1 && obj.Access != nil {
			return obj.Access.MatchString(key[index+1:], predicate)
		}
	case "Trunk":
		if index != -1 && obj.Trunk != nil {
			return obj.Trunk.MatchString(key[index+1:], predicate)
		}
	}
	return false
}

func (obj *VLANInfo) GetField(key string) (interface{}, error) {
	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "Access":
		if obj.Access != nil {
			if index != -1 {
				return obj.Access.GetField(key[index+1:])
			} else {
				return obj.Access, nil
			}
		}
	case "Trunk":
		if obj.Trunk != nil {
			if index != -1 {
				return obj.Trunk.GetField(key[index+1:])
			} else {
				return obj.Trunk, nil
			}
		}

	}
	return nil, getter.ErrFieldNotFound
}

func (obj *VLANTrunkInfo) GetFieldBool(key string) (bool, error) {
	return false, getter.ErrFieldNotFound
}

func (obj *VLANTrunkInfo) GetFieldInt64(key string) (int64, error) {
	switch key {
	case "NativeVLANVID":
		return int64(obj.NativeVLANVID), nil
	}
	return 0, getter.ErrFieldNotFound
}

func (obj *VLANTrunkInfo) GetFieldString(key string) (string, error) {
	return "", getter.ErrFieldNotFound
}

func (obj *VLANTrunkInfo) GetFieldKeys() []string {
	return []string{
		"AcceptedVIDs",
		"NativeVLANVID",
	}
}

func (obj *VLANTrunkInfo) MatchBool(key string, predicate getter.BoolPredicate) bool {
	return false
}

func (obj *VLANTrunkInfo) MatchInt64(key string, predicate getter.Int64Predicate) bool {
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	return false
}

func (obj *VLANTrunkInfo) MatchString(key string, predicate getter.StringPredicate) bool {
	return false
}

func (obj *VLANTrunkInfo) GetField(key string) (interface{}, error) {
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}

	first := key
	index := strings.Index(key, ".")
	if index != -1 {
		first = key[:index]
	}

	switch first {
	case "AcceptedVIDs":
		if obj.AcceptedVIDs != nil {
			if index != -1 {
			} else {
				var results []interface{}
				for _, obj := range obj.AcceptedVIDs {
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
