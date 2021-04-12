package snmplldp

import (
	"fmt"
	"strings"
	"time"

	snmp "github.com/gosnmp/gosnmp"
)

type lldpInfo struct {
	name        string
	desc        string
	chassisID   string
	remoteTable map[string]*lldpRemote
}

type lldpRemote struct {
	name      string
	desc      string
	chassisID string
}

type lldpObjName int

const (
	localName lldpObjName = iota
	localDesc
	localChassisID
	remoteName
	remoteDesc
	remoteChassisID
)

func getLLDPInfo(host string, port uint16, community string) (*lldpInfo, error) {
	snmpClient := &snmp.GoSNMP{
		Version: snmp.Version2c,
		Timeout: time.Duration(10) * time.Second,

		Target:    host,
		Port:      port,
		Community: community,
	}

	info := &lldpInfo{}

	err := snmpClient.Connect()
	if err != nil {
		return info, err
	}
	defer snmpClient.Conn.Close()

	// Object/OID table
	localObjects := map[lldpObjName]string{
		localName:      "1.0.8802.1.1.2.1.3.3.0",
		localDesc:      "1.0.8802.1.1.2.1.3.4.0",
		localChassisID: "1.0.8802.1.1.2.1.3.2.0",
	}

	// Retrieve local system info
	for object, oid := range localObjects {
		result, err := snmpClient.Get([]string{oid})
		if err != nil {
			return info, err
		}

		switch object {
		case localName:
			info.name = string(result.Variables[0].Value.([]byte))
		case localDesc:
			info.desc = string(result.Variables[0].Value.([]byte))
		case localChassisID:
			hexrep := fmt.Sprintf("% x", result.Variables[0].Value)
			info.chassisID = strings.ReplaceAll(hexrep, " ", ":")
		}

	}

	info.remoteTable, err = getRemoteTable(snmpClient)
	return info, err
}

func getRemoteTable(client *snmp.GoSNMP) (map[string]*lldpRemote, error) {
	// Column (object)/OID table
	columns := map[lldpObjName]string{
		remoteName:      "1.0.8802.1.1.2.1.4.1.1.9",
		remoteDesc:      "1.0.8802.1.1.2.1.4.1.1.10",
		remoteChassisID: "1.0.8802.1.1.2.1.4.1.1.5",
	}

	table := make(map[string]*lldpRemote)
	var results []snmp.SnmpPDU
	var err error = nil

	for column, oid := range columns {
		results, err = client.WalkAll(oid)
		if err != nil {
			return table, err
		}

		// Retrieve the rows (instances) of the column (object)
		for _, e := range results {
			row := strings.ReplaceAll(e.Name, oid, "")
			if table[row] == nil {
				table[row] = &lldpRemote{}
			}

			switch column {
			case remoteName:
				table[row].name = string(e.Value.([]byte))
			case remoteDesc:
				table[row].desc = string(e.Value.([]byte))
			case remoteChassisID:
				hexrep := fmt.Sprintf("% x", e.Value)
				table[row].chassisID = strings.ReplaceAll(hexrep, " ", ":")
			}

		}
	}

	return table, err
}
