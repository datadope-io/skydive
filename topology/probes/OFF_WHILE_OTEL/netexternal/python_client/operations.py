import sgqlc.types
import sgqlc.operation
import netexternal

_schema = netexternal
_schema_root = _schema.netexternal

__all__ = ('Operations',)


def mutation_add_switch_vlan():
    _op = sgqlc.operation.Operation(_schema_root.mutation_type, name='addSwitchVLAN')
    _op_add_switch = _op.add_switch(input={'Name': 'swMAD01x', 'MAC': '00:11:22:33:44:55', 'Vendor': 'Cisco', 'Model': '5600', 'Interfaces': [{'Name': 'ge/1/0', 'Aggregation': False, 'VLAN': {'Mode': 'ACCESS', 'VID': [123]}}, {'Name': 'tr/1/0', 'Aggregation': False, 'VLAN': {'Mode': 'TRUNK', 'VID': [123, 124, 125], 'NativeVID': 20}}]})
    _op_add_switch.updated()
    _op_add_switch.interface_updated()
    _op_add_switch.id()
    return _op


def mutation_add_router_ips():
    _op = sgqlc.operation.Operation(_schema_root.mutation_type, name='addRouterIPs')
    _op_add_router = _op.add_router(input={'Name': 'rBCN01x', 'Vendor': 'Cisco', 'Model': '9600', 'Interfaces': [{'Name': 'tr/1/0', 'Aggregation': False, 'VLAN': {'Mode': 'TRUNK', 'VID': [123, 124, 125], 'NativeVID': 20}, 'IP': {'IP': '10.0.1.1', 'Mask': '255.255.255.0'}}, {'Name': 'tr/1/1', 'Aggregation': False, 'VLAN': {'Mode': 'TRUNK', 'VID': [99, 199], 'NativeVID': 20}, 'IP': {'IP': '192.168.1.1', 'Mask': '255.255.0.0', 'VRF': 'MGMT'}}, {'Name': 'tr/2/2', 'Aggregation': False, 'VLAN': {'Mode': 'TRUNK', 'VID': [90], 'NativeVID': 20}, 'IP': {'IP': '10.0.2.1', 'Mask': '255.255.255.0'}}]})
    _op_add_router.updated()
    _op_add_router.interface_updated()
    _op_add_router.id()
    return _op


class Mutation:
    add_switch_vlan = mutation_add_switch_vlan()
    add_router_ips = mutation_add_router_ips()


class Operations:
    mutation = Mutation
