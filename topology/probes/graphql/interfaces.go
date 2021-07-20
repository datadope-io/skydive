package graphql

import (
	"fmt"
	"time"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/topology/probes/graphql/model"
)

const (
	TypeRouter    = "router"
	TypeSwitch    = "switch"
	TypeInterface = "interface"
	TypeServer    = "server"

	MetaKeyRelationType   = "RelationType"
	RelationTypeOwnership = "ownership"
	RelationConnectsTo    = "ConnectsTo"

	MetaKeyType        = "Type"
	MetaKeyName        = "Name"
	MetaKeyVendor      = "Vendor"
	MetaKeyModel       = "Model"
	MetaKeyAggregation = "Aggregation"
)

func (r *Resolver) addDeviceWithInterfaces(
	name string,
	metadata graph.Metadata,
	interfaces []*model.InterfaceInput,
	createdAt *time.Time,
) (node *graph.Node, updated bool, interfaceUpdated bool, err error) {

	id := str2GraphID(name)
	node = r.Graph.GetNode(id)

	// Create, if it does not exists.
	// Return internal ID if it exists.
	if node == nil {
		node, err = r.newNode(id, metadata, createdAt)
		if err != nil {
			return node, updated, interfaceUpdated, err
		}
	} else {
		// Update metadata if needed
		revisionPreUpdate := node.Revision

		err = r.Graph.SetMetadata(node, metadata)
		if err != nil {
			logging.GetLogger().Errorf("unable to update node %+v: %v", node, err)
			return node, updated, interfaceUpdated, fmt.Errorf("error updating node: %v", err)
		}
		// If SetMetadata modifies metadata, it will increase revision number.
		// We use this to know if switch metadata has been modified and notify the user.
		if revisionPreUpdate != node.Revision {
			updated = true
		}
	}

	// Interfaces
	interfaceUpdated, err = r.createInterfaces(node, interfaces, createdAt)
	if err != nil {
		logging.GetLogger().Errorf("creating interfaces for node %+v: %v", node, err)
		return nil, false, false, fmt.Errorf("creating interfaces: %v", err)
	}

	return node, updated, interfaceUpdated, nil
}

func (r *Resolver) createIf2IfEdge(
	srcDevice string,
	srcInterface string,
	dstDevice string,
	dstInterface string,
	createdAt *time.Time,
) (edge *graph.Edge, err error) {
	// Get source node
	s := fmt.Sprintf("%s__%s", srcDevice, srcInterface)
	srcNode := r.Graph.GetNode(str2GraphID(s))

	// Get destination node
	s = fmt.Sprintf("%s__%s", dstDevice, dstInterface)
	dstNode := r.Graph.GetNode(str2GraphID(s))

	edge, err = r.newEdge(srcNode, dstNode, map[string]interface{}{
		MetaKeyRelationType: RelationConnectsTo,
	}, createdAt)
	if err != nil {
		return nil, fmt.Errorf("unable to link (%+v) with (%+v): %v", srcNode, dstNode, err)
	}

	return edge, nil
}

func (r *Resolver) createInterfaces(
	device *graph.Node,
	interfaces []*model.InterfaceInput,
	createdAt *time.Time,
) (updated bool, err error) {
	// Iterate over user defined interfaces, creating or updating while needed
	for _, iface := range interfaces {
		nodeName := device.Metadata[MetaKeyName]

		ifaceMetadata := map[string]interface{}{
			MetaKeyName: iface.Name,
			"Device":    nodeName,
			MetaKeyType: "interface",
		}

		// Generate ID: sha256("device__ifName")
		s := fmt.Sprintf("%s__%s", nodeName, iface.Name)
		ifID := str2GraphID(s)

		ifNode := r.Graph.GetNode(ifID)
		if ifNode == nil {
			// Create the interface and assign the node to "iface" var
			ifNode, err = r.newNode(ifID, ifaceMetadata, createdAt)
			if err != nil {
				logging.GetLogger().Errorf("unable to create interface %+v: %v", ifaceMetadata, err)
				return updated, fmt.Errorf("unable to create interface: %v", err)
			}
		} else {
			revisionPreUpdateIface := ifNode.Revision
			errS := r.Graph.SetMetadata(ifNode, ifaceMetadata)
			if errS != nil {
				logging.GetLogger().Errorf("unable to update interface metadata %+v: %v", ifNode, err)
				return updated, fmt.Errorf("unable to update interface metadata %+v: %v", ifNode, err)
			}
			if revisionPreUpdateIface != ifNode.Revision {
				updated = true
			}
		}

		if iface.Aggregation != nil && *iface.Aggregation != "" {
			aggregation := *iface.Aggregation
			ifaceMetadata[MetaKeyAggregation] = aggregation
			r.createAggrIface(aggregation, device, ifNode, createdAt)
		} else {
			aggregation := iface.Name + "__NoAggregation"
			ifaceMetadata[MetaKeyAggregation] = aggregation
			r.createAggrIface(aggregation, device, ifNode, createdAt)
		}
	}

	return updated, nil
}

func (r *Resolver) createAggrIface(
	aggregation string,
	device *graph.Node,
	ifNode *graph.Node,
	createdAt *time.Time,
) error {
	var err error
	nodeName := device.Metadata[MetaKeyName]

	aggMetadata := map[string]interface{}{
		MetaKeyName: aggregation,
		"Device":    nodeName,
		MetaKeyType: "aggregation",
	}

	s := fmt.Sprintf("%s__%s", nodeName, aggregation)
	aggID := str2GraphID(s)

	aggNode := r.Graph.GetNode(aggID)
	if aggNode == nil {
		// Create the interface and assign the node to "iface" var
		aggNode, err = r.newNode(aggID, aggMetadata, createdAt)
		if err != nil {
			fmt.Printf("unable to create aggregate interface %+v: %v\n", aggMetadata, err)
			return fmt.Errorf("unable to create aggregate interface: %v", err)
		}
	}

	// Link device with aggregate interface
	if !r.Graph.AreLinked(device, aggNode, nil) {
		_, err = r.newEdge(device, aggNode, map[string]interface{}{
			MetaKeyRelationType: RelationTypeOwnership,
		}, createdAt)
		if err != nil {
			fmt.Printf("unable to link device (%+v) to aggregation (%+v): %v\n", device, aggNode, err)
			return fmt.Errorf("unable to link device (%+v) to aggregation (%+v): %v", device, aggNode, err)
		}
	}

	// Link interface with aggregate interface
	if !r.Graph.AreLinked(aggNode, ifNode, nil) {
		_, err = r.newEdge(aggNode, ifNode, map[string]interface{}{
			MetaKeyRelationType: RelationTypeOwnership,
		}, createdAt)
		if err != nil {
			fmt.Printf("unable to link interface (%+v) to aggregation (%+v): %v\n", ifNode, aggNode, err)
			return fmt.Errorf("unable to link interface (%+v) to aggregation (%+v): %v", ifNode, aggNode, err)
		}
	}

	return nil
}
