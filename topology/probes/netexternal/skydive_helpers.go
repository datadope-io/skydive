package netexternal

import (
	"crypto/sha256"
	"fmt"
	"time"

	uuid "github.com/nu7hatch/gouuid"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/topology/probes/netexternal/model"
)

const (
	// PrefixOriginName defines the prefix set in nodes/edges Origin field
	PrefixOriginName = "netexternal."

	TypeRouter    = "router"
	TypeSwitch    = "switch"
	TypeInterface = "interface"
	TypeServer    = "server"

	MetadataRelationTypeKey = "RelationType"
	RelationTypeOwnership   = "ownership"

	MetadataTypeKey   = "Type"
	MetadataNameKey   = "Name"
	MetadataVendorKey = "Vendor"
	MetadataModelKey  = "Model"
)

// newEdge create a new edge in Skydive with a custom Origin field
func (r *Resolver) newEdge(n *graph.Node, c *graph.Node, m graph.Metadata, createdAt *time.Time) (*graph.Edge, error) {
	u, _ := uuid.NewV5(uuid.NamespaceOID, []byte(n.ID+c.ID))
	i := graph.Identifier(u.String())

	var timestamp graph.Time
	if createdAt != nil {
		timestamp = graph.Time(*createdAt)
	} else {
		timestamp = graph.TimeUTC()
	}
	e := graph.CreateEdge(i, n, c, m, timestamp, r.Graph.GetHost(), PrefixOriginName+r.Graph.GetOrigin())

	if err := r.Graph.AddEdge(e); err != nil {
		return nil, err
	}
	return e, nil
}

// newNode create a new edge in Skydive with a custom Origin field
func (r *Resolver) newNode(id graph.Identifier, m graph.Metadata, createdAt *time.Time) (*graph.Node, error) {
	var timestamp graph.Time
	if createdAt != nil {
		timestamp = graph.Time(*createdAt)
	} else {
		timestamp = graph.TimeUTC()
	}
	n := graph.CreateNode(id, m, timestamp, r.Graph.GetHost(), PrefixOriginName+r.Graph.GetOrigin())

	if err := r.Graph.AddNode(n); err != nil {
		return nil, err
	}
	return n, nil
}

func (r *Resolver) updateNode(id string, m graph.Metadata, modifiedAt *time.Time) (*graph.Node, error) {
	old_node := r.Graph.GetNode(graph.Identifier(id))
	if old_node == nil {
		return nil, fmt.Errorf("Node doesn't exist")
	}

	new_node := graph.CreateNode(old_node.ID, m, old_node.CreatedAt, old_node.Host, old_node.Origin)
	// Increment revision
	new_node.Revision = old_node.Revision + 1

	err := r.Graph.NodeUpdated(new_node)
	if err != nil {
		return nil, fmt.Errorf("Updating node")
	}

	return new_node, nil
}

func (r *Resolver) createInterfaces(
	device *graph.Node,
	interfaces []*model.InterfaceInput,
	createdAt *time.Time,
) (updated bool, err error) {

	// Iterate over user defined interfaces, creating or updating while needed
	for _, iface := range interfaces {
		ifaceMetadata := map[string]interface{}{
			MetaKeyName: iface.Name,
			MetaKeyType: "interface",
		}

		if iface.Aggregation != nil {
			ifaceMetadata[MetaKeyAggregation] = iface.Aggregation
		}

		// Generate ID: sha256("device__ifName")
		nodeName := device.Metadata[MetadataNameKey]
		idStr := fmt.Sprintf("%s__%s", nodeName, iface.Name)
		h := sha256.Sum256([]byte(idStr))
		id := graph.Identifier(fmt.Sprintf("%x", h))

		node := r.Graph.GetNode(id)
		if node == nil {
			// Create the interface and assign the node to "iface" var
			node, err = r.newNode(id, ifaceMetadata, createdAt)
			if err != nil {
				logging.GetLogger().Errorf("unable to create interface %+v: %v", ifaceMetadata, err)
				return updated, fmt.Errorf("unable to create interface: %v", err)
			}
		} else {
			revisionPreUpdateIface := node.Revision
			errS := r.Graph.SetMetadata(node, ifaceMetadata)
			if errS != nil {
				logging.GetLogger().Errorf("unable to update interface metadata %+v: %v", node, err)
				return updated, fmt.Errorf("unable to update interface metadata %+v: %v", node, err)
			}
			if revisionPreUpdateIface != node.Revision {
				updated = true
			}
		}

		// Link node with interface
		if !r.Graph.AreLinked(device, node, nil) {
			_, err = r.newEdge(device, node, map[string]interface{}{
				MetadataRelationTypeKey: RelationTypeOwnership,
			}, createdAt)
			if err != nil {
				return updated, fmt.Errorf("unable to link node (%+v) to interface (%+v): %v", node, node, err)
			}
		}

	}

	return updated, nil
}

func (r *Resolver) addNodeWithInterfaces(
	name string,
	metadata graph.Metadata,
	interfaces []*model.InterfaceInput,
	createdAt *time.Time,
) (node *graph.Node, updated bool, interfaceUpdated bool, err error) {

	h := sha256.Sum256([]byte(name))
	id := graph.Identifier(fmt.Sprintf("%x", h))
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
