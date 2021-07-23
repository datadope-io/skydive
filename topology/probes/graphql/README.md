# GraphQL probe

This probe exposes a [GraphQL](https://graphql.org/) API to access, modify and create Skydive data.

It's primary objetive is to allow integration of third parties with a more powerful integration than using the REST API.

``mutations`` is the name used by GraphQL to denote calls that will produce a modification (like POST/PUT/DELETE in HTTP).

``queries`` are to obtain data (like GET in HTTP).


## Configuration
To enable this probe we should add it to the analyzer probes list:
```yaml
analyzer:
  topology:
    probes:
      - graphql
```

By default it will listen on ``0.0.0.0:8080``. This could be changed in ``analyzer.topology.graphql.listen``, eg.:
```
analyzer:
  topology:
    graphql:
      listen: 127.0.0.1:5005
```


## Utilization
Once we have Skydive running with the GraphQL probe, we can browse to the http endpoint to access the [GraphQL Playground](https://github.com/graphql/graphql-playground).

This web interface help us building the GraphQL queries to interact with the probe. Documentation for the API is shown is this interface.

## Modules

Currently there are two modules implemented in this probe: ``net`` and ``event``.

To clearly distinguish between calls to different modules in the GraphQL scheme, each module will prefix its queries/mutations with its name. For example, the ``event`` module have the ``event_Add`` and ``event_Close`` mutations.

### Net
This module allows to create network devices (routers, switches) and its interfaces.

The module will check if the device, or device's interfaces, already exists and create/update them.

The returned data contains the device ID, if it has been created or updated. Also shows info for interfaces and edges.

Example creating a router with some interface:
```
mutation {
  net_AddDevice(input: {
    Name: "RouterA",
    Type: Router,
    Vendor:"Cisco"
    Model:"5502",
  }) {
    Device {
      ID
      NodeCreated
      NodeUpdated
    }
  }
  iface1:net_AddInterface(input: {
    DeviceSelector:{
      GremlinQuery:"G.V().Has('Name','RouterA','DeviceType','Router')"
    }
    Interface:{
      Name:"eth0"
      Ethernet: {
        VLAN: {
          Access: {
            AcceptedVIDs:[101,102]
            NativeVLANVID:100
          }
        }
      }
      IP:["192.168.0.1/24", "10.0.0.1/24"]
    }
  }) {
    Interface {
      ID
      NodeCreated
      NodeUpdated
    }
  }
  iface2:net_AddInterface(input: {
    DeviceSelector:{
      GremlinQuery:"G.V().Has('Name','RouterA','DeviceType','Router')"
    }
    Interface:{
      Name:"gi0/0"
    }
      Aggregation:"trunk0"
    }) {
    Interface {
      ID
      NodeCreated
      NodeUpdated
    }
  }
  iface3:net_AddInterface(input: {
    DeviceSelector:{
      GremlinQuery:"G.V().Has('Name','RouterA','DeviceType','Router')"
    }
    Interface:{
      Name:"gi0/1"
    }
      Aggregation:"trunk0"
    }) {
    Interface {
      ID
      NodeCreated
      NodeUpdated
    }
  }
  iface4:net_AddAggregation(input: {
    DeviceSelector:{
      GremlinQuery:"G.V().Has('Name','RouterA','DeviceType','Router')"
    }
    Aggregation:{
      Name:"trunk0"
      Ethernet:{
        VLAN:{
          Trunk:{
            VID:400
          }
        }
      }
      IP:["10.0.1.1/24","172.17.0.1/32"]
    }
    }) {
    Aggregation {
      ID
      NodeCreated
      NodeUpdated
    }
  }
}
```

This call will create 5 nodes and 6 edges as shown here:
```
routerA──┬───(ownership)──►eth0
         │
         ├───(ownership)──►gi0/0◄──(aggregation)─────┐
         │                                           │
         ├───(ownership)──►gi0/1◄──(aggregation)─────┤
         │                                           │
         └───(ownership)──►trunk0────────────────────┘
```
In this case we are creating physical or aggregation interfaces, each one one could have defined ethernet and/or IP data.

If there is some problem creating/updating the nodes, the call will return an error.


Is it also possible to create _logical_ interfaces. These are virtual interefaces connected to some VLAN and running on top of some physical or aggregation interface.

Create a new router with a physical device and a logical interface on top. In this case, we are going to create the elements in different calls, using the ID of the previous call to define the device or interface needed.

Create the router:
```
mutation {
  net_AddDevice(input: {
    Name: "RouterB",
    Type: Router,
  }) {
    Device {
      ID
      NodeCreated
      NodeUpdated
    }
  }
}
```

Create the physical interface:
```
mutation {
  net_AddInterface(input: {
    DeviceSelector:{
      ID:"0d9d4c1a-927b-52a8-61d6-a337e05b31a5"
    }
    Interface:{
      Name:"eth0"
    }
  }) {
    Interface {
      ID
      NodeCreated
      NodeUpdated
    }
  }
}
```

Create the logical interface:
```
mutation {
  net_AddLogicalInterface(input:{
    InterfaceSelector:{
      ID:"f5afd600-8bc8-5029-6857-24cb60aec14d"
    }
    Interface:{
      Name:"eth0.300"
      Ethernet:{
        VLAN:{
          Trunk:{
            VID:300
          }
        }
      }
      IP:["192.168.100.100/24"]
    }
  }) {
    LogicalInterface {
      ID
      NodeCreated
      NodeUpdated
    }
  }
}
```

We end up with:
```
routerB────(ownership)────►eth0◄──(use)──eth0.300
```

Finally we use ``net_AddIf2IfLink`` to link ``routerA`` and ``routerB``:
```
mutation {
  net_AddIf2IfLink(input:{
    SrcInterface:{
      GremlinQuery:"G.V().Has('Name','RouterA','DeviceType','Router').Out('Name','gi0/0')"
    }
    DstInterface:{
      GremlinQuery:"G.V().Has('Name','RouterB','DeviceType','Router').Out('Name','eth0')"
    }
  }) {
    InterfaceLink {
      ID
      EdgeCreated
      EdgeUpdated
    }
  }
}
```

Creating an edge between routers' physical interfaces:
```
routerA────(ownership)────►gi0/0───(ConnectsTo)───►eth0◄───(ownership)───routerB
```




### Event
This module allows the creation and deletion of events. The reasoning is to be able to accept events from external monitoring systems as ElasticSearch or Zabbix.
This data will be stored in ``Metadata.Events``, eg.:
```
"Metadata": {
  "Name": "eth1",
  "Type": "interface",
  "Device": "routerA",
  "Events": {
    "zabbix_46712823": {
      "TTL": 0,
      "Level": "INFO",
      "Payload": [
        {
          "Key": "Description",
          "Value": "Too many interface errors"
        },
        {
          "Key": "count",
          "Value": "123"
        }
      ],
      "CreatedAt": 1627377674229,
      "UpdatedAt": 1627377674229
    }
  }
}
```

With this integration we can use Skydive as a central point where we can show certain topology with the events, or alarms, affecting the nodes shown.

Some systems, like Zabbix, use the concept of ``Alarm``, as an event that fires that alarm and another event that closes that alarm.
This systems will use ``event_Add`` and ``event_Close``.

Others systems could generate only events, like Elastic-ML. For this cases, we could use the field ``TTL`` to expire an event and ``event_Add`` to keep the event _alive_.

For example, we configure Elastic-ML with a ``bucket_span`` of 5 minutes. Each 5' Elastic-ML will call ``event_Add`` if there is issues in the data analyzed.
Let's say Elastic-ML sends events at :00, :05 and :10, and then stops sending events. If ``TTL`` is set to 5', around :15 this module will delete the event (see next section to understand how this deletion works).

Example call to create an event:
```
mutation {
  event_Add(input:{
      GremlinQuery:"G.V().Has('Name','routerA').Out('Name','aggA')"
      ID:"elasticml_Traffic-RX"
      Level:WARNING
      TTL:300
      Payload:[
        {Key:"score", Value:"45.3"},
        {Key:"probability", Value:"0.4"},
      ]
  }) {
    ID
  }
}
```
It will return the modified node ID if the node has been added successfully or an error otherwise.

Example of a successfull call:
```
{
  "data": {
    "event_Add": {
      "ID": "0701ab3c-9e37-502f-6fc2-eaff69857331"
    }
  }
}
```

Example of an error:
```
{
  "errors": [
    {
      "message": "expecting only one node, gremlin query returned 0",
      "path": [
        "event_Add"
      ]
    }
  ],
  "data": null
}
```

We could also create several events in different nodes with a single call:
```
mutation {
  event_AddMultiple(input:[
    {
      GremlinQuery:"G.V().Has('Name','routerA').Out('Name','aggA')"
      ID:"elasticml_Traffic-RX"
      Level:WARNING
      TTL:300
      Payload:[
        {Key:"score", Value:"45.3"},
        {Key:"probability", Value:"0.4"},
      ]
    },
    {
      GremlinQuery:"G.V().Has('Name','routerA').Out('Type','interface_aggregation').Out('Name','eth1')"
      ID:"zabbix_46712823"
      Level:INFO
      Payload:[
        {Key:"Description", Value:"Too many interface errors"},
        {Key:"count", Value:"123"},
      ]
    }
  ]) {
    Ok
    Fail
    Errors
  }
}
```

In this case it will only return an error if there is only one input event and it has failed.

Otherwise it will return a count of ok and failed events, and a list of errors, eg.:
```
{
  "data": {
    "event_AddMultiple": {
      "Ok": 1,
      "Fail": 1,
      "Errors": [
        "Event 0 (Query: 'G.V().Has('Name','routerA').Out('Name','aggAX')', ID: 'elasticml_Traffic-RX'): expecting only one node, gremlin query returned 0"
      ]
    }
  }
}
```

#### Automatic event deletion
The event module has a _garbage collector_ that runs each 2' (by default, it could be changed in ``analyzer.topology.graphql.event.garbage_collector_interval``).

When this _garbage collector_ runs, it will iterate over all nodes that have ``Metadata.Events`` defined.
For each event it will check if the current time is higher than UpdatedAt+TTL, in that case, the event will be deleted.
