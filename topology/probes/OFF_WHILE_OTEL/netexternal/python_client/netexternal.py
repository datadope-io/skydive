import sgqlc.types


netexternal = sgqlc.types.Schema()



########################################################################
# Scalars and Enumerations
########################################################################
Boolean = sgqlc.types.Boolean

Float = sgqlc.types.Float

ID = sgqlc.types.ID

Int = sgqlc.types.Int

String = sgqlc.types.String

class VLAN_MODE(sgqlc.types.Enum):
    __schema__ = netexternal
    __choices__ = ('ACCESS', 'TRUNK')



########################################################################
# Input Objects
########################################################################
class InterfaceIPInput(sgqlc.types.Input):
    __schema__ = netexternal
    __field_names__ = ('ip', 'mask', 'vrf')
    ip = sgqlc.types.Field(sgqlc.types.non_null(String), graphql_name='IP')
    mask = sgqlc.types.Field(sgqlc.types.non_null(String), graphql_name='Mask')
    vrf = sgqlc.types.Field(String, graphql_name='VRF')


class InterfaceInput(sgqlc.types.Input):
    __schema__ = netexternal
    __field_names__ = ('name', 'aggregation', 'vlan', 'ip')
    name = sgqlc.types.Field(sgqlc.types.non_null(String), graphql_name='Name')
    aggregation = sgqlc.types.Field(Boolean, graphql_name='Aggregation')
    vlan = sgqlc.types.Field('InterfaceVLANInput', graphql_name='VLAN')
    ip = sgqlc.types.Field(InterfaceIPInput, graphql_name='IP')


class InterfaceVLANInput(sgqlc.types.Input):
    __schema__ = netexternal
    __field_names__ = ('mode', 'vid', 'native_vid')
    mode = sgqlc.types.Field(sgqlc.types.non_null(VLAN_MODE), graphql_name='Mode')
    vid = sgqlc.types.Field(sgqlc.types.non_null(sgqlc.types.list_of(sgqlc.types.non_null(Int))), graphql_name='VID')
    native_vid = sgqlc.types.Field(Int, graphql_name='NativeVID')


class RouterInput(sgqlc.types.Input):
    __schema__ = netexternal
    __field_names__ = ('name', 'vendor', 'model', 'interfaces')
    name = sgqlc.types.Field(sgqlc.types.non_null(String), graphql_name='Name')
    vendor = sgqlc.types.Field(String, graphql_name='Vendor')
    model = sgqlc.types.Field(String, graphql_name='Model')
    interfaces = sgqlc.types.Field(sgqlc.types.list_of(InterfaceInput), graphql_name='Interfaces')


class SwitchInput(sgqlc.types.Input):
    __schema__ = netexternal
    __field_names__ = ('name', 'mac', 'vendor', 'model', 'interfaces')
    name = sgqlc.types.Field(sgqlc.types.non_null(String), graphql_name='Name')
    mac = sgqlc.types.Field(sgqlc.types.non_null(String), graphql_name='MAC')
    vendor = sgqlc.types.Field(String, graphql_name='Vendor')
    model = sgqlc.types.Field(String, graphql_name='Model')
    interfaces = sgqlc.types.Field(sgqlc.types.list_of(InterfaceInput), graphql_name='Interfaces')


class VLANInput(sgqlc.types.Input):
    __schema__ = netexternal
    __field_names__ = ('vid', 'name')
    vid = sgqlc.types.Field(sgqlc.types.non_null(Int), graphql_name='VID')
    name = sgqlc.types.Field(String, graphql_name='Name')



########################################################################
# Output Objects and Interfaces
########################################################################
class AddRouterPayload(sgqlc.types.Type):
    __schema__ = netexternal
    __field_names__ = ('id', 'updated', 'interface_updated')
    id = sgqlc.types.Field(sgqlc.types.non_null(String), graphql_name='ID')
    updated = sgqlc.types.Field(sgqlc.types.non_null(Boolean), graphql_name='Updated')
    interface_updated = sgqlc.types.Field(sgqlc.types.non_null(Boolean), graphql_name='InterfaceUpdated')


class AddSwitchPayload(sgqlc.types.Type):
    __schema__ = netexternal
    __field_names__ = ('id', 'updated', 'interface_updated')
    id = sgqlc.types.Field(sgqlc.types.non_null(String), graphql_name='ID')
    updated = sgqlc.types.Field(sgqlc.types.non_null(Boolean), graphql_name='Updated')
    interface_updated = sgqlc.types.Field(sgqlc.types.non_null(Boolean), graphql_name='InterfaceUpdated')


class AddVLANPayload(sgqlc.types.Type):
    __schema__ = netexternal
    __field_names__ = ('id',)
    id = sgqlc.types.Field(sgqlc.types.non_null(String), graphql_name='ID')


class Mutation(sgqlc.types.Type):
    __schema__ = netexternal
    __field_names__ = ('add_vlan', 'add_switch', 'add_router')
    add_vlan = sgqlc.types.Field(sgqlc.types.non_null(AddVLANPayload), graphql_name='addVLAN', args=sgqlc.types.ArgDict((
        ('input', sgqlc.types.Arg(sgqlc.types.non_null(VLANInput), graphql_name='input', default=None)),
))
    )
    add_switch = sgqlc.types.Field(sgqlc.types.non_null(AddSwitchPayload), graphql_name='addSwitch', args=sgqlc.types.ArgDict((
        ('input', sgqlc.types.Arg(sgqlc.types.non_null(SwitchInput), graphql_name='input', default=None)),
))
    )
    add_router = sgqlc.types.Field(sgqlc.types.non_null(AddRouterPayload), graphql_name='addRouter', args=sgqlc.types.ArgDict((
        ('input', sgqlc.types.Arg(sgqlc.types.non_null(RouterInput), graphql_name='input', default=None)),
))
    )


class Query(sgqlc.types.Type):
    __schema__ = netexternal
    __field_names__ = ('version',)
    version = sgqlc.types.Field(sgqlc.types.non_null(String), graphql_name='version')



########################################################################
# Unions
########################################################################

########################################################################
# Schema Entry Points
########################################################################
netexternal.query_type = Query
netexternal.mutation_type = Mutation
netexternal.subscription_type = None

