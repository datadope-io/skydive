// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

import (
	"fmt"
	"io"
	"strconv"
	"time"
)

// Return value when creating a new Router.
type AddRouterPayload struct {
	// Internal ID for the node in Skydive
	ID string `json:"ID"`
	// Return true if a router with this name already exists
	// and metadata values have been updated.
	Updated bool `json:"Updated"`
	// Return true if some interface of the router has been updated
	InterfaceUpdated bool `json:"InterfaceUpdated"`
}

// Return value when creating a new Switch.
type AddSwitchPayload struct {
	// Internal ID for the node in Skydive
	ID string `json:"ID"`
	// Return true if a switch with this name already exists
	// and metadata values have been updated.
	Updated bool `json:"Updated"`
	// Return true if some interface of the switch has been updated
	InterfaceUpdated bool `json:"InterfaceUpdated"`
}

// Return value when creating a new VLAN.
type AddVLANPayload struct {
	// Internal ID for the node in Skydive
	ID string `json:"ID"`
}

// Datos de entrada para crear un evento y asociarlo a un nodo
type EventInput struct {
	// Nombre del nodo al que queremos añadir un evento.
	// Solo algunos tipos de nodos pueden recibir eventos, son aquellos que se pueden
	// identificar unívocamente por su Name+Type: servers, software, routers, switches, etc
	// Otros como las interfaces, tienen el mismo Name+Type y se diferencian por su relación
	// con otros nodos del primer tipo. A estos últimos no podemos añadir alarmas directamente.
	Name string `json:"Name"`
	// Cadena obligatoria con la que podamos relacionar eventos generados por el mismo origen.
	// Por ejemplo, para Zabbix, la pkey será el triggerID, con el que logramos identificar
	// que distintos eventos proceden del mismo origen.
	// En el caso de alarmas enviadas por Elastic-ML, tendrá que ser una combinación de
	// nombre de la función y otros parámetros con los que logremos distinguir que eventos
	// con los "mismos" enviados en distintos momentos en el tiempo.
	Pkey string `json:"Pkey"`
	// Para los nodos tipo Router o Switch, se permite pasar, de manera opcional, el nombre
	// de la interfaz involucrada en el evento.
	// En caso de que dicha interfaz exista asociada al router/switch, se meterá la alarma
	// en el nodo interfaz. En caso contrario, la alarma se asociará al router/switch.
	Interface *string `json:"Interface"`
	// Fecha del evento. En caso de no estar definido se usará la fecha actual
	Time *time.Time `json:"Time"`
	// Texto con formato JSON donde añadimos información extra del evento.
	// Aquí podemos añadir, por ejemplo, la criticidad del evento, un texto descriptivo, etc
	Metadata string `json:"Metadata"`
}

// Respuesta enviada al usuario al crear un evento
type EventPayload struct {
	// Return if the addEvent mutation was processed correctly
	Ok bool `json:"Ok"`
	// En caso de ok:false, retornamos un mensaje de error.
	Error *string `json:"Error"`
}

// Level 3 (IP) configuration for interfaces
// TODO: not tested with IPv6
type InterfaceIPInput struct {
	IP string `json:"IP"`
	// IP mask, format: 255.255.255.0
	Mask string `json:"Mask"`
	// This L3 interface belongs to this VRF.
	Vrf *string `json:"VRF"`
}

// Network interface.
// Could represent a port in a switch, a router or Server.
type InterfaceInput struct {
	// Name of the interface, eg.: ge/0/1
	// This parameter, with the link to some network element, is the primary key.
	Name string `json:"Name"`
	// Set true for virtual interfaces aggregatting other physical interfaces (LACP)
	Aggregation *bool `json:"Aggregation"`
	// VLAN configuration for this interface
	Vlan *InterfaceVLANInput `json:"VLAN"`
	// IP configuration
	// TODO: pueden tener varias IPs?
	IP *InterfaceIPInput `json:"IP"`
}

// VLAN configuration for an interface
type InterfaceVLANInput struct {
	Mode VlanMode `json:"Mode"`
	// For ACCESS mode, define one element with the.
	// For TRUNK mode, define all possible VIDs allowed.
	Vid []int `json:"VID"`
	// Set the native VLAN for this interface
	NativeVid *int `json:"NativeVID"`
}

// IP route.
// Network could be specified with CIDR or IP+mask
type Route struct {
	// Specify the network matching using CIDR
	Cidr *string `json:"CIDR"`
	// Specify the network matching using IP+mask
	IP   *string `json:"IP"`
	Mask *string `json:"Mask"`
	// Optinal name for this particular route
	Name *string `json:"Name"`
	// Use a device as the next hop instead of an IP
	DeviceNextHop *string `json:"DeviceNextHop"`
	// IP address for next hop
	NextHop *string `json:"NextHop"`
}

// Values to create a new Router
type RouterInput struct {
	// Router name. Used as the primary key.
	Name string `json:"Name"`
	// Vendor of the switch, eg.: Cisco, Juniper
	Vendor *string `json:"Vendor"`
	// Switch model, eg.: CBS250-8P-E-2G
	Model *string `json:"Model"`
	// Interfaces/ports of the router
	// Undeclared intefaces already present are not deleted.
	// Interfaces already present are updated.
	Interfaces []*InterfaceInput `json:"Interfaces"`
	// Routing table. Each element represents a different VRF.
	RoutingTable []*VRFRouteTable `json:"RoutingTable"`
}

// Values to create a new Switch.
type SwitchInput struct {
	// Switch name. Used as the primary key.
	Name string `json:"Name"`
	// MAC address of the device.
	// Allowed formats https://pkg.go.dev/net#ParseMAC
	Mac *string `json:"MAC"`
	// Vendor of the switch, eg.: Cisco, Juniper
	Vendor *string `json:"Vendor"`
	// Switch model, eg.: CBS250-8P-E-2G
	Model *string `json:"Model"`
	// Interfaces/ports of the switch
	// Undeclared intefaces already present are not deleted.
	// Interfaces already present are updated.
	Interfaces []*InterfaceInput `json:"Interfaces"`
	// IP routes for switches with this feature.
	// Switches does not have VRF, so we will create only one VRFRouteTable without VRF name.
	RoutingTable []*VRFRouteTable `json:"RoutingTable"`
}

// Values to create a new VLAN.
type VLANInput struct {
	// VLAN ID. Used as the primary key.
	Vid int `json:"VID"`
	// Optional VLAN name
	Name *string `json:"Name"`
}

type VRFRouteTable struct {
	// VRF name. If not defined will take value "default"
	Vrf    *string  `json:"VRF"`
	Routes []*Route `json:"Routes"`
}

// VLAN modes
type VlanMode string

const (
	VlanModeAccess VlanMode = "ACCESS"
	VlanModeTrunk  VlanMode = "TRUNK"
)

var AllVlanMode = []VlanMode{
	VlanModeAccess,
	VlanModeTrunk,
}

func (e VlanMode) IsValid() bool {
	switch e {
	case VlanModeAccess, VlanModeTrunk:
		return true
	}
	return false
}

func (e VlanMode) String() string {
	return string(e)
}

func (e *VlanMode) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = VlanMode(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid VLAN_MODE", str)
	}
	return nil
}

func (e VlanMode) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
