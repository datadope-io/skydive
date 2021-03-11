package proccon

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// Remove comment to change logging level to debug
	// logging.InitLogging("id", true, []*logging.LoggerConfig{logging.NewLoggerConfig(logging.NewStdioBackend(os.Stdout), "5", "UTF-8")})
	os.Exit(m.Run())
}

func newGraph(t *testing.T) *graph.Graph {
	b, err := graph.NewMemoryBackend()
	if err != nil {
		t.Error(err.Error())
	}

	return graph.NewGraph("testhost", b, "analyzer.testhost")
}

// sendAgentData simulates the HTTP post of an external agent
func sendAgentData(t *testing.T, p Probe, data []byte) {
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(data))
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()

	p.ServeHTTP(rr, req)

	// Check HTTP 200 and empty body
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v. Error: %v",
			status, http.StatusOK, rr.Body.String())
	}
}

// getTCPConn return the list of connections stored in this node Metadata
// Return an empty slice if metadata key does not exists or is empty
func getTCPConn(node *graph.Node) (conn []string, err error) {
	data, err := node.Metadata.GetField(MetadataTCPConnKey)
	if err != nil {
		logging.GetLogger().Warningf("Node %+v does not have metadata key %s", node, MetadataTCPConnKey)
		return conn, nil
	}

	for c := range *data.(*NetworkInfo) {
		conn = append(conn, c)
	}

	return conn, err
}

// getListenEndpoints return the list of listen endpoints stored in this node Metadata
// Return an empty slice if metadata key does not exists or is empty
func getListenEndpoints(node *graph.Node) (listen []string, err error) {
	data, err := node.Metadata.GetField(MetadataListenEndpointKey)
	if err != nil {
		logging.GetLogger().Warningf("Node %+v does not have metadata key %s", node, MetadataListenEndpointKey)
		return listen, nil
	}

	for l := range *data.(*NetworkInfo) {
		listen = append(listen, l)
	}

	return listen, err
}

// TestCreateServerNodeSoftwareOthers receives a metric with an unkown host, it should create a new Server node in the Graph with that name,
// a software node called "others", a edge between them and append the connection info to that software node
func TestCreateServerNodeSoftwareOthers(t *testing.T) {
	// GIVEN
	p := Probe{}
	p.graph = newGraph(t)

	metricServerName := "hostFoo"
	metricSoftwareCmdline := "nc -kl 8000"
	metricConnections := "1.2.3.4:80,9.9.9.9:53"
	metricListen := "192.168.1.36:8000,192.168.1.22:8000"
	agentData := []byte(fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": 1603890543
    }
  ]
}`, metricConnections, metricListen, metricSoftwareCmdline, metricServerName))

	// WHEN
	sendAgentData(t, p, agentData)

	// THEN
	// Check if Server node has been created correctly
	serverNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeServer))
	servers := p.graph.GetNodes(serverNodeFilter)
	if len(servers) == 0 {
		t.Fatal("Server node not created")
	} else if len(servers) > 1 {
		t.Error("Too many Server nodes created")
	}

	server := servers[0]
	serverName, err := server.Metadata.GetFieldString(MetadataNameKey)
	if err != nil {
		t.Errorf("Server node created but without Metadata.Name")
	}

	if serverName != metricServerName {
		t.Errorf("Server node created, but with wrong name, %s != %s (expected)", serverName, metricServerName)
	}

	// Check if Software node 'others' exists and have the right name and connection info
	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	softwares := p.graph.GetNodes(softwareNodeFilter)
	if len(softwares) == 0 {
		t.Fatal("Software node not created")
	} else if len(softwares) > 1 {
		t.Error("Too many Software nodes created")
	}

	software := softwares[0]
	softwareName, err := software.Metadata.GetFieldString(MetadataNameKey)
	if err != nil {
		t.Errorf("Software node created but without Metadata.Name")
	}

	if softwareName != OthersSoftwareNode {
		t.Errorf("Software node created, but with wrong name: %v", softwareName)
	}

	// The software node should have two revisions:
	//  - creating the node
	//  - adding TCPConn and TCPListen to that empty node
	assert.Equal(t, int64(2), software.Revision)

	softwareTCPConn, err := getTCPConn(software)
	if err != nil {
		t.Errorf("Not able to get TCP connection info")
	}
	assert.ElementsMatch(t, softwareTCPConn, strings.Split(metricConnections, ","))

	softwareListenEndpoints, err := getListenEndpoints(software)
	if err != nil {
		t.Errorf("Not able to get TCP listen endpoints")
	}
	assert.ElementsMatch(t, softwareListenEndpoints, strings.Split(metricListen, ","))

	// Check the edge between these two nodes
	hasSoftwareEdgesFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataRelationTypeKey, RelationTypeHasSoftware))
	edges := p.graph.GetEdges(hasSoftwareEdgesFilter)
	if len(edges) == 0 {
		t.Fatal("has_software edge not created")
	} else if len(edges) > 1 {
		t.Error("Too many has_software edges created")
	}

	edge := edges[0]
	if edge.Parent != server.ID {
		t.Errorf("Edge parent is not Server node")
	}
	if edge.Child != software.ID {
		t.Errorf("Edge child is not Software node")
	}
}

// TestPresentServerCreateSoftwareToOthers receives a metric of a server node already in the graph, but without a "others" software node. It should
// create that "others" software node and append the connection info.
func TestPresentServerCreateSoftwareToOthers(t *testing.T) {
	// GIVEN
	p := Probe{}

	p.graph = newGraph(t)

	givenServerName := "hostFoo"

	serverNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: givenServerName,
		MetadataTypeKey: MetadataTypeServer,
	})
	if err != nil {
		t.Errorf("Unable to create server %s", givenServerName)
	}

	// WHEN
	metricServerName := givenServerName
	metricSoftwareCmdline := "nc -kl 8000"
	metricConnections := "1.2.3.4:80,9.9.9.9:53"
	metricListen := "192.168.1.36:8000,192.168.1.22:8000"
	agentData := []byte(fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": 1603890543
    }
  ]
}`, metricConnections, metricListen, metricSoftwareCmdline, metricServerName))

	sendAgentData(t, p, agentData)

	// THEN
	// Check if Software node 'others' exists and have the right name and connection info
	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	softwares := p.graph.GetNodes(softwareNodeFilter)
	if len(softwares) == 0 {
		t.Fatal("Software node not created")
	} else if len(softwares) > 1 {
		t.Error("Too many Software nodes created")
	}

	software := softwares[0]
	softwareName, err := software.Metadata.GetFieldString(MetadataNameKey)
	if err != nil {
		t.Errorf("Software node created but without Metadata.Name")
	}

	if softwareName != OthersSoftwareNode {
		t.Errorf("Software node created, but with wrong name: %v", softwareName)
	}

	softwareTCPConn, err := getTCPConn(software)
	if err != nil {
		t.Errorf("Not able to get TCP connection info")
	}
	assert.ElementsMatch(t, softwareTCPConn, strings.Split(metricConnections, ","))

	softwareListenEndpoints, err := getListenEndpoints(software)
	if err != nil {
		t.Errorf("Not able to get TCP listen endpoints")
	}
	assert.ElementsMatch(t, softwareListenEndpoints, strings.Split(metricListen, ","))

	// Check there ir an edge connecting software to server node
	e := p.graph.GetNodeEdges(software, graph.NewElementFilter(filters.NewTermStringFilter(MetadataRelationTypeKey, RelationTypeHasSoftware)))
	assert.Len(t, e, 1)
	assert.Equal(t, e[0].Parent, serverNode.ID)
}

// TestFillOthersSoftwareNode given a present server and empty 'others' software node, add the connection info to that node
func TestFillOthersSoftwareNode(t *testing.T) {
	// GIVEN
	p := Probe{}

	p.graph = newGraph(t)

	givenServerName := "hostFoo"

	givenNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: givenServerName,
		MetadataTypeKey: MetadataTypeServer,
	})
	if err != nil {
		t.Errorf("Unable to create server %s", givenServerName)
	}

	givenOtherNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: OthersSoftwareNode,
		MetadataTypeKey: MetadataTypeSoftware,
	})
	if err != nil {
		t.Error("Unable to create software others")
	}

	_, err = p.graph.NewEdge("", givenNode, givenOtherNode, graph.Metadata{
		MetadataRelationTypeKey: RelationTypeHasSoftware,
	})
	if err != nil {
		t.Errorf("Unable to create edge between server %s and software others", givenServerName)
	}

	// WHEN
	metricServerName := "hostFoo"
	metricSoftwareCmdline := "nc -kl 8000"
	metricConnections := "1.2.3.4:80,9.9.9.9:53"
	metricListen := "192.168.1.36:8000,192.168.1.22:8000"
	agentData := []byte(fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": 1603890543
    }
  ]
}`, metricConnections, metricListen, metricSoftwareCmdline, metricServerName))

	sendAgentData(t, p, agentData)

	// THEN
	// Check if Software node 'others' exists and have the right name and connection info
	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	softwares := p.graph.GetNodes(softwareNodeFilter)
	if len(softwares) == 0 {
		t.Fatal("Software node not created")
	} else if len(softwares) > 1 {
		t.Error("Too many Software nodes created")
	}

	software := softwares[0]
	softwareName, err := software.Metadata.GetFieldString(MetadataNameKey)
	if err != nil {
		t.Errorf("Software node created but without Metadata.Name")
	}

	if softwareName != OthersSoftwareNode {
		t.Errorf("Software node created, but with wrong name: %v", softwareName)
	}

	softwareTCPConn, err := getTCPConn(software)
	if err != nil {
		t.Errorf("Not able to get TCP connection info")
	}
	assert.ElementsMatch(t, softwareTCPConn, strings.Split(metricConnections, ","))

	softwareListenEndpoints, err := getListenEndpoints(software)
	if err != nil {
		t.Errorf("Not able to get TCP listen endpoints")
	}
	assert.ElementsMatch(t, softwareListenEndpoints, strings.Split(metricListen, ","))
}

// TestMetricDateIsUsed checks that CreatedAt and UpdatedAt fields are set to the metric timestamp for new metrics
func TestMetricDateIsUsed(t *testing.T) {
	// GIVEN
	p := Probe{}

	p.graph = newGraph(t)

	givenServerName := "hostFoo"

	givenNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: givenServerName,
		MetadataTypeKey: MetadataTypeServer,
	})
	if err != nil {
		t.Errorf("Unable to create server %s", givenServerName)
	}

	givenOtherNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: OthersSoftwareNode,
		MetadataTypeKey: MetadataTypeSoftware,
	})
	if err != nil {
		t.Error("Unable to create software others")
	}

	_, err = p.graph.NewEdge("", givenNode, givenOtherNode, graph.Metadata{
		MetadataRelationTypeKey: RelationTypeHasSoftware,
	})
	if err != nil {
		t.Errorf("Unable to create edge between server %s and software others", givenServerName)
	}

	// WHEN
	metricServerName := "hostFoo"
	metricSoftwareCmdline := "nc -kl 8000"
	metricConnections := "1.2.3.4:80"
	metricListen := "192.168.1.36:8000"
	var metricTimestamp int64 = 1555555555
	agentData := []byte(fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": %d
    }
  ]
}`, metricConnections, metricListen, metricSoftwareCmdline, metricServerName, metricTimestamp))

	sendAgentData(t, p, agentData)

	// THEN
	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	software := p.graph.GetNodes(softwareNodeFilter)[0]

	metadataTCPConnRaw, _ := software.Metadata.GetField(MetadataTCPConnKey)
	connMetadata := (*metadataTCPConnRaw.(*NetworkInfo))[metricConnections]

	metadataListenEndpointsRaw, _ := software.Metadata.GetField(MetadataListenEndpointKey)
	listenMetadata := (*metadataListenEndpointsRaw.(*NetworkInfo))[metricListen]

	expectedMetadata := ProcInfo{
		CreatedAt: metricTimestamp * 1000, // CreatedAt is in milliseconds
		UpdatedAt: metricTimestamp * 1000, // UpdatedAt is in milliseconds
		Revision:  1,
	}

	assert.Equal(t, connMetadata, expectedMetadata)
	assert.Equal(t, listenMetadata, expectedMetadata)
}

// TestMultipleMetricsToOtherOnlyOneRevision if several metrics are received in the same packet and are going to the "others" software,
// the node.Revision field should only increase by one. Network info from both connetions should be added to "others"
func TestMultipleMetricsToOtherOnlyOneRevision(t *testing.T) {
	// GIVEN
	p := Probe{}

	p.graph = newGraph(t)

	givenServerName := "hostFoo"

	givenNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: givenServerName,
		MetadataTypeKey: MetadataTypeServer,
	})
	if err != nil {
		t.Errorf("Unable to create server %s", givenServerName)
	}

	givenOtherNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: OthersSoftwareNode,
		MetadataTypeKey: MetadataTypeSoftware,
	})
	if err != nil {
		t.Error("Unable to create software others")
	}

	_, err = p.graph.NewEdge("", givenNode, givenOtherNode, graph.Metadata{
		MetadataRelationTypeKey: RelationTypeHasSoftware,
	})
	if err != nil {
		t.Errorf("Unable to create edge between server %s and software others", givenServerName)
	}

	// WHEN
	metricServerName := "hostFoo"
	metricSoftwareCmdline := "nc -kl 8000"
	metricConnections := "1.2.3.4:80"
	metricListen := "192.168.1.36:8000"

	metricSoftwareCmdline2 := "nc -kl 8888"
	metricConnections2 := "1.2.3.4:88"
	metricListen2 := "192.168.1.36:8888"

	metricSoftwareCmdline3 := "nc -kl 9999"
	metricConnections3 := "1.2.3.4:99"
	metricListen3 := "192.168.1.36:9999"

	var metricTimestamp int64 = 1555555555
	agentData := fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": %d
    },
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": %d
    },
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": %d
    }
  ]
}`, metricConnections, metricListen, metricSoftwareCmdline, metricServerName, metricTimestamp,
		metricConnections2, metricListen2, metricSoftwareCmdline2, metricServerName, metricTimestamp,
		metricConnections3, metricListen3, metricSoftwareCmdline3, metricServerName, metricTimestamp,
	)

	// First send
	sendAgentData(t, p, []byte(agentData))

	// THEN
	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	software := p.graph.GetNodes(softwareNodeFilter)[0]

	// The software node should have two revisions:
	//  - creating the node
	//  - adding TCPConn and TCPListen to that empty node
	assert.Equal(t, int64(2), software.Revision)

	metadataTCPConnRaw, _ := software.Metadata.GetField(MetadataTCPConnKey)
	connMetadata := (*metadataTCPConnRaw.(*NetworkInfo))
	assert.Len(t, connMetadata, 3)

	metadataListenEndpointsRaw, _ := software.Metadata.GetField(MetadataListenEndpointKey)
	listenMetadata := (*metadataListenEndpointsRaw.(*NetworkInfo))
	assert.Len(t, listenMetadata, 3)
}

// TestTwoMetricsTwoOthersNode given a metric message with two metrics from different servers (tag.host), it should create
// two different Server node and two different "others" Software node
func TestTwoMetricsTwoOthersNode(t *testing.T) {
	// GIVEN
	p := Probe{}
	p.graph = newGraph(t)

	metricServerName := "hostFoo"
	metricSoftwareCmdline := "nc -kl 8000"
	metricConnections := "1.2.3.4:80"
	metricListen := "192.168.1.36:8000"

	metricServerName2 := "hostBar"
	metricSoftwareCmdline2 := "nc -kl 8888"
	metricConnections2 := "1.2.3.4:88"
	metricListen2 := "192.168.1.36:8888"

	var metricTimestamp int64 = 1555555555
	agentData := fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": %d
    },
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": %d
    }
  ]
}`, metricConnections, metricListen, metricSoftwareCmdline, metricServerName, metricTimestamp,
		metricConnections2, metricListen2, metricSoftwareCmdline2, metricServerName2, metricTimestamp,
	)

	// WHEN
	sendAgentData(t, p, []byte(agentData))

	// THEN
	serverNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeServer))
	servers := p.graph.GetNodes(serverNodeFilter)
	assert.Len(t, servers, 2)

	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	softwares := p.graph.GetNodes(softwareNodeFilter)
	assert.Len(t, softwares, 2)
}

// TestMetricDateIsUsedWhenUpdating consecutives updates for the same metric should increase Revision and use the metric timestamp in the UpdatedAt field
func TestMetricDateIsUsedWhenUpdating(t *testing.T) {
	// GIVEN
	p := Probe{}

	p.graph = newGraph(t)

	givenServerName := "hostFoo"

	givenNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: givenServerName,
		MetadataTypeKey: MetadataTypeServer,
	})
	if err != nil {
		t.Errorf("Unable to create server %s", givenServerName)
	}

	givenOtherNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: OthersSoftwareNode,
		MetadataTypeKey: MetadataTypeSoftware,
	})
	if err != nil {
		t.Error("Unable to create software others")
	}

	_, err = p.graph.NewEdge("", givenNode, givenOtherNode, graph.Metadata{
		MetadataRelationTypeKey: RelationTypeHasSoftware,
	})
	if err != nil {
		t.Errorf("Unable to create edge between server %s and software others", givenServerName)
	}

	// WHEN
	metricServerName := "hostFoo"
	metricSoftwareCmdline := "nc -kl 8000"
	metricConnections := "1.2.3.4:80"
	metricListen := "192.168.1.36:8000"
	var metricTimestamp int64 = 1555555555
	agentData := fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": %d
    }
  ]
}`, metricConnections, metricListen, metricSoftwareCmdline, metricServerName, metricTimestamp)

	// First send
	sendAgentData(t, p, []byte(agentData))

	// Second send
	var secondMetricTimestamp int64 = 1666666666
	secondAgentData := strings.Replace(agentData, strconv.Itoa(int(metricTimestamp)), strconv.Itoa(int(secondMetricTimestamp)), 1)
	sendAgentData(t, p, []byte(secondAgentData))

	// THEN
	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	software := p.graph.GetNodes(softwareNodeFilter)[0]

	metadataTCPConnRaw, _ := software.Metadata.GetField(MetadataTCPConnKey)
	connMetadata := (*metadataTCPConnRaw.(*NetworkInfo))[metricConnections]

	metadataListenEndpointsRaw, _ := software.Metadata.GetField(MetadataListenEndpointKey)
	listenMetadata := (*metadataListenEndpointsRaw.(*NetworkInfo))[metricListen]

	expectedMetadata := ProcInfo{
		CreatedAt: metricTimestamp * 1000,       // CreatedAt is in milliseconds
		UpdatedAt: secondMetricTimestamp * 1000, // UpdatedAt is in milliseconds
		Revision:  2,
	}

	assert.Equal(t, connMetadata, expectedMetadata)
	assert.Equal(t, listenMetadata, expectedMetadata)
}

// TestMetricInvalidTimestamp check that invalid timestamps lead to use time.Now() in the UpdatedAt/CreatedAt fields
func TestMetricInvalidTimestamp(t *testing.T) {
	// GIVEN
	p := Probe{}

	p.graph = newGraph(t)

	givenServerName := "hostFoo"

	givenNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: givenServerName,
		MetadataTypeKey: MetadataTypeServer,
	})
	if err != nil {
		t.Errorf("Unable to create server %s", givenServerName)
	}

	givenOtherNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: OthersSoftwareNode,
		MetadataTypeKey: MetadataTypeSoftware,
	})
	if err != nil {
		t.Error("Unable to create software others")
	}

	_, err = p.graph.NewEdge("", givenNode, givenOtherNode, graph.Metadata{
		MetadataRelationTypeKey: RelationTypeHasSoftware,
	})
	if err != nil {
		t.Errorf("Unable to create edge between server %s and software others", givenServerName)
	}

	// WHEN
	metricServerName := "hostFoo"
	metricSoftwareCmdline := "nc -kl 8000"
	metricConnections := "1.2.3.4:80"
	metricListen := "192.168.1.36:8000"
	var metricTimestamp int64 = 1e11
	agentData := fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": %d
    }
  ]
}`, metricConnections, metricListen, metricSoftwareCmdline, metricServerName, metricTimestamp)

	// First send
	sendAgentData(t, p, []byte(agentData))

	// THEN
	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	software := p.graph.GetNodes(softwareNodeFilter)[0]

	metadataTCPConnRaw, _ := software.Metadata.GetField(MetadataTCPConnKey)
	connMetadata := (*metadataTCPConnRaw.(*NetworkInfo))[metricConnections]

	metadataListenEndpointsRaw, _ := software.Metadata.GetField(MetadataListenEndpointKey)
	listenMetadata := (*metadataListenEndpointsRaw.(*NetworkInfo))[metricListen]

	assert.NotEqual(t, connMetadata.CreatedAt, metricTimestamp*1000)
	assert.NotEqual(t, connMetadata.UpdatedAt, metricTimestamp*1000)
	assert.NotEqual(t, listenMetadata.CreatedAt, metricTimestamp*1000)
	assert.NotEqual(t, listenMetadata.UpdatedAt, metricTimestamp*1000)
}

// TestFillOthersSoftwareNodeWithConnPrefix if the received metrics has the tag connPrefix, IPs stored in Skydive should prefix that value
func TestFillOthersSoftwareNodeWithConnPrefix(t *testing.T) {
	// GIVEN
	p := Probe{}

	p.graph = newGraph(t)

	givenServerName := "hostFoo"

	givenNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: givenServerName,
		MetadataTypeKey: MetadataTypeServer,
	})
	if err != nil {
		t.Errorf("Unable to create server %s", givenServerName)
	}

	givenOtherNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: OthersSoftwareNode,
		MetadataTypeKey: MetadataTypeSoftware,
	})
	if err != nil {
		t.Error("Unable to create software others")
	}

	_, err = p.graph.NewEdge("", givenNode, givenOtherNode, graph.Metadata{
		MetadataRelationTypeKey: RelationTypeHasSoftware,
	})
	if err != nil {
		t.Errorf("Unable to create edge between server %s and software others", givenServerName)
	}

	// WHEN
	metricServerName := "hostFoo"
	metricSoftwareCmdline := "nc -kl 8000"
	metricConnections := "1.2.3.4:80,9.9.9.9:53"
	metricListen := "192.168.1.36:8000,192.168.1.22:8000"
	connPrefix := "foobar-"
	agentData := []byte(fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc",
				"conn_prefix": "%s"
      },
      "timestamp": 1603890543
    }
  ]
}`, metricConnections, metricListen, metricSoftwareCmdline, metricServerName, connPrefix))

	sendAgentData(t, p, agentData)

	// THEN
	// Check if Software node 'others' exists and have the right name and connection info
	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	softwares := p.graph.GetNodes(softwareNodeFilter)
	if len(softwares) == 0 {
		t.Fatal("Software node not created")
	} else if len(softwares) > 1 {
		t.Error("Too many Software nodes created")
	}

	software := softwares[0]
	softwareTCPConn, _ := getTCPConn(software)
	assert.ElementsMatch(t, softwareTCPConn, []string{"foobar-1.2.3.4:80", "foobar-9.9.9.9:53"})

	softwareListenEndpoints, _ := getListenEndpoints(software)
	assert.ElementsMatch(t, softwareListenEndpoints, []string{"foobar-192.168.1.36:8000", "foobar-192.168.1.22:8000"})
}

// TestNewMetricUpdateNetworkMetadata given a present 'others' software node with some data, if a new metric is received, it should update the network metadata
func TestNewMetricUpdateNetworkMetadata(t *testing.T) {
	// GIVEN
	p := Probe{}

	p.graph = newGraph(t)

	givenServerName := "hostFoo"
	givenOthersSoftwareTCPConnections := []string{"1.2.3.4:80"}
	givenOthersSoftwareListenEndpoints := []string{"192.168.0.1:22"}

	givenNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: givenServerName,
		MetadataTypeKey: MetadataTypeServer,
	})
	if err != nil {
		t.Errorf("Unable to create server %s", givenServerName)
	}

	givenOtherNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: OthersSoftwareNode,
		MetadataTypeKey: MetadataTypeSoftware,
	})
	if err != nil {
		t.Error("Unable to create software others")
	}

	_, err = p.graph.NewEdge("", givenNode, givenOtherNode, graph.Metadata{
		MetadataRelationTypeKey: RelationTypeHasSoftware,
	})
	if err != nil {
		t.Errorf("Unable to create edge between server %s and software others", givenServerName)
	}

	// This function handles its own lock
	metric := Metric{
		Fields: Fields{
			Conn:   strings.Join(givenOthersSoftwareTCPConnections, ","),
			Listen: strings.Join(givenOthersSoftwareListenEndpoints, ","),
		},
	}
	err = p.addNetworkInfo(givenOtherNode, []Metric{metric})
	if err != nil {
		t.Error("Adding network connections to others Software node")
	}

	// This created other nod should have 2 revisions
	//  - creating the node
	//  - adding TCPConn and TCPListen to that empty node
	assert.Equal(t, int64(2), givenOtherNode.Revision)

	// WHEN
	agentData := []byte(fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "nc -kl 8000",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": 1603890543
    }
  ]
}`, strings.Join(givenOthersSoftwareTCPConnections, ","), strings.Join(givenOthersSoftwareListenEndpoints, ","), givenServerName))

	time.Sleep(time.Millisecond) // To be able to see a difference between UpdatedAt and CreatedAt
	sendAgentData(t, p, agentData)

	// THEN
	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	software := p.graph.GetNodes(softwareNodeFilter)[0]

	// After handling the metrics, node should have increased its revision by two
	//  - adding TCPConn to existant Metadata
	//  - adding TCPListen to existant Metadata
	assert.Equal(t, int64(4), givenOtherNode.Revision)

	softwareTCPConn, err := software.Metadata.GetField(MetadataTCPConnKey)
	if err != nil {
		t.Fatalf("Software 'others' must have the %v key in metadata", MetadataTCPConnKey)
	}

	procInfoTCPConnPtr, ok := softwareTCPConn.(*NetworkInfo)
	assert.True(t, ok)
	procInfoTCPConn := *procInfoTCPConnPtr

	// It should have just one connection
	assert.Len(t, procInfoTCPConn, 1)

	conn := procInfoTCPConn[givenOthersSoftwareTCPConnections[0]]

	// This connection should have the revision metadata field set to 1, as it have received a new metric after creation
	assert.Equal(t, int64(2), conn.Revision)
	// It should shown an update time newer than creating time
	assert.Greater(t, conn.UpdatedAt, conn.CreatedAt)

	softwareListenEndpoint, err := software.Metadata.GetField(MetadataListenEndpointKey)
	if err != nil {
		t.Fatalf("Software 'others' must have the %v key in metadata", MetadataListenEndpointKey)
	}

	procInfoListenEndpointPtr, ok := softwareListenEndpoint.(*NetworkInfo)
	assert.True(t, ok)
	procInfoListenEndpoint := *procInfoListenEndpointPtr

	// It should have just one listener
	assert.Len(t, procInfoListenEndpoint, 1)

	listen := procInfoListenEndpoint[givenOthersSoftwareListenEndpoints[0]]

	// This listener should have the revision metadata field set to 1, as it have received a new metric after creation
	assert.Equal(t, int64(2), listen.Revision)
	// It should shown an update time newer than creating time
	assert.Greater(t, listen.UpdatedAt, listen.CreatedAt)
}

// TestAppendConnectionInfoToOthersSoftwareNode given a present 'others' software node with some data, check that new data is appended and old data
// is kept
func TestAppendConnectionInfoToOthersSoftwareNode(t *testing.T) {
	// GIVEN
	p := Probe{}

	p.graph = newGraph(t)

	givenServerName := "hostFoo"
	givenOthersSoftwareTCPConnections := []string{"1.2.3.4:80", "8.8.8.8:443"}
	givenOthersSoftwareListenEndpoints := []string{"192.168.0.1:22", "10.0.1.1:22"}

	p.graph.Lock()
	givenNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: givenServerName,
		MetadataTypeKey: MetadataTypeServer,
	})
	if err != nil {
		t.Errorf("Unable to create server %s", givenServerName)
	}

	givenOtherNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: OthersSoftwareNode,
		MetadataTypeKey: MetadataTypeSoftware,
	})
	if err != nil {
		t.Error("Unable to create software others")
	}

	_, err = p.graph.NewEdge("", givenNode, givenOtherNode, graph.Metadata{
		MetadataRelationTypeKey: RelationTypeHasSoftware,
	})
	if err != nil {
		t.Errorf("Unable to create edge between server %s and software others", givenServerName)
	}
	p.graph.Unlock()

	// This function handles its own lock
	metric := Metric{
		Fields: Fields{
			Conn:   strings.Join(givenOthersSoftwareTCPConnections, ","),
			Listen: strings.Join(givenOthersSoftwareListenEndpoints, ","),
		},
	}
	err = p.addNetworkInfo(givenOtherNode, []Metric{metric})
	if err != nil {
		t.Error("Adding network connections to others Software node")
	}

	// WHEN
	metricConnections := []string{}
	metricListen := []string{"192.168.1.36:8000", "192.168.1.22:8000", "10.0.1.1:22"}
	agentData := []byte(fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "nc -kl 8000",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": 1603890543
    }
  ]
}`, strings.Join(metricConnections, ","), strings.Join(metricListen, ","), givenServerName))

	sendAgentData(t, p, agentData)

	// THEN
	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	software := p.graph.GetNodes(softwareNodeFilter)[0]

	softwareTCPConn, err := getTCPConn(software)
	if err != nil {
		t.Errorf("Not able to get TCP connection info")
	}
	// TCP connections should be the union of the present ones with those in the metric
	expectedTCPConn := append(givenOthersSoftwareTCPConnections, metricConnections...)
	assert.ElementsMatch(t, softwareTCPConn, expectedTCPConn)

	softwareListenEndpoints, err := getListenEndpoints(software)
	if err != nil {
		t.Errorf("Not able to get TCP listen endpoints")
	}
	// Listen endpoints should be the union of the present ones with those in the metric
	expectedListenEndpoints := []string{"192.168.0.1:22", "10.0.1.1:22", "192.168.1.36:8000", "192.168.1.22:8000"}
	assert.ElementsMatch(t, softwareListenEndpoints, expectedListenEndpoints)
}

//
// TestFillKnownSoftwareNode given a Server node and a liked Software node, if the metric received matches the cmdline of the software node, the
// connection info should be appended to that software node
func TestFillKnownSoftwareNode(t *testing.T) {
	// GIVEN
	p := Probe{}

	p.graph = newGraph(t)

	givenSoftwareName := "PostgreSQL"
	cmdline := "/usr/bin/postgres -D /var/lib/postgres/data"
	givenServerName := "hostFoo"
	givenSoftwareTCPConnections := []string{}
	givenSoftwareListenEndpoints := []string{"192.168.0.1:5432", "10.0.1.1:5432"}

	p.graph.Lock()
	givenNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: givenServerName,
		MetadataTypeKey: MetadataTypeServer,
	})
	if err != nil {
		t.Errorf("Unable to create server %s", givenServerName)
	}

	givenSWNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey:    givenSoftwareName,
		MetadataTypeKey:    MetadataTypeSoftware,
		MetadataCmdlineKey: cmdline,
	})
	if err != nil {
		t.Error("Unable to create software others")
	}

	_, err = p.graph.NewEdge("", givenNode, givenSWNode, graph.Metadata{
		MetadataRelationTypeKey: RelationTypeHasSoftware,
	})
	if err != nil {
		t.Errorf("Unable to create edge between server %s and software others", givenServerName)
	}
	p.graph.Unlock()

	// This function handles its own lock
	metric := Metric{
		Fields: Fields{
			Conn:   strings.Join(givenSoftwareTCPConnections, ","),
			Listen: strings.Join(givenSoftwareListenEndpoints, ","),
		},
	}
	err = p.addNetworkInfo(givenSWNode, []Metric{metric})
	if err != nil {
		t.Error("Adding network connections to others Software node")
	}

	// WHEN
	metricConnections := []string{}
	metricListen := []string{"192.168.1.36:8000", "192.168.1.22:8000", "10.0.1.1:22"}
	agentData := []byte(fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": 1603890543
    }
  ]
}`, strings.Join(metricConnections, ","), strings.Join(metricListen, ","), cmdline, givenServerName))

	sendAgentData(t, p, agentData)

	// THEN
	softwareNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	software := p.graph.GetNodes(softwareNodeFilter)[0]

	softwareName, _ := software.Metadata.GetFieldString(MetadataNameKey)
	assert.Equal(t, givenSoftwareName, softwareName)

	softwareTCPConn, err := getTCPConn(software)
	if err != nil {
		t.Errorf("Not able to get TCP connection info")
	}
	// TCP connections should be the union of the present ones with those in the metric
	expectedTCPConn := metricConnections
	assert.ElementsMatch(t, softwareTCPConn, expectedTCPConn)

	softwareListenEndpoints, err := getListenEndpoints(software)
	if err != nil {
		t.Errorf("Not able to get TCP listen endpoints")
	}
	// Listen endpoints should be the union of the present ones with those in the metric
	expectedListenEndpoints := []string{"192.168.0.1:5432", "10.0.1.1:5432", "192.168.1.36:8000", "192.168.1.22:8000", "10.0.1.1:22"}
	assert.ElementsMatch(t, softwareListenEndpoints, expectedListenEndpoints)
}

// TestClearOldConnections check if old connections stored in the metadata are deleted correctly by the appropiate function
func TestClearOldConnections(t *testing.T) {
	// GIVEN
	p := Probe{}
	p.graph = newGraph(t)

	metricServerName := "hostFoo"
	metricSoftwareCmdline := "nc -kl 8000"
	metricConnections := "1.2.3.4:80,9.9.9.9:53"
	metricListen := "192.168.1.36:8000,192.168.1.22:8000"
	agentData := []byte(fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": 1603890543
    }
  ]
}`, metricConnections, metricListen, metricSoftwareCmdline, metricServerName))

	sendAgentData(t, p, agentData)
	swNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	software := p.graph.GetNodes(swNodeFilter)[0]

	// WHEN
	// This should delete all connections, as the threshold time is in the future
	p.removeOldNetworkInformation(software, time.Now().Add(time.Hour))

	// THEN
	softwareTCPConn, err := getTCPConn(software)
	if err != nil {
		t.Errorf("Not able to get TCP connection info")
	}
	assert.Empty(t, softwareTCPConn)

	softwareListenEndpoints, err := getListenEndpoints(software)
	if err != nil {
		t.Errorf("Not able to get TCP listen endpoints")
	}
	assert.Empty(t, softwareListenEndpoints)
}

// TestClearOldConnectionsKeepNewerConnections for a given node with old and new connections, check that clearing old connections
// do not delete new connections
func TestClearOldConnectionsKeepNewerConnections(t *testing.T) {
	// GIVEN
	p := Probe{}
	p.graph = newGraph(t)

	software, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey: OthersSoftwareNode,
		MetadataTypeKey: MetadataTypeSoftware,
		MetadataTCPConnKey: &NetworkInfo{
			"1.1.1.1:80": {
				CreatedAt: 0,
				UpdatedAt: 0,
				Revision:  1,
			},
			"1.2.3.4:80": {
				CreatedAt: graph.TimeNow().UnixMilli(),
				UpdatedAt: graph.TimeNow().UnixMilli(),
				Revision:  1,
			},
		},
		MetadataListenEndpointKey: &NetworkInfo{
			"1.1.1.1:80": {
				CreatedAt: 0,
				UpdatedAt: 0,
				Revision:  1,
			},
			"1.2.3.4:80": {
				CreatedAt: graph.TimeNow().UnixMilli(),
				UpdatedAt: graph.TimeNow().UnixMilli(),
				Revision:  1,
			},
		},
	})
	if err != nil {
		t.Error("Unable to create software others")
	}

	// WHEN
	// Should remove old connections but no the new ones
	p.removeOldNetworkInformation(software, time.Now().Add(-time.Hour))

	// THEN
	softwareTCPConn, err := getTCPConn(software)
	if err != nil {
		t.Errorf("Not able to get TCP connection info")
	}
	assert.Len(t, softwareTCPConn, 1)

	softwareListenEndpoints, err := getListenEndpoints(software)
	if err != nil {
		t.Errorf("Not able to get TCP listen endpoints")
	}
	assert.Len(t, softwareListenEndpoints, 1)
}

// TestDoNotPanicIfInvalidTCPConnOrTCPListenDataType checks that parsing invalid data do not panic
func TestDoNotPanicIfInvalidTCPConnOrTCPListenDataType(t *testing.T) {
	// GIVEN
	p := Probe{}
	p.graph = newGraph(t)

	software, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey:           OthersSoftwareNode,
		MetadataTypeKey:           MetadataTypeSoftware,
		MetadataTCPConnKey:        "",
		MetadataListenEndpointKey: "",
	})
	if err != nil {
		t.Error("Unable to create software others")
	}

	// WHEN
	// Should remove old connections but no the new ones
	p.removeOldNetworkInformation(software, time.Now().Add(-time.Hour))
}

// TestCleanSoftwareNodes check if garbage collector function delete correctly old connections in all software nodes
func TestClearSoftwareNodes(t *testing.T) {
	// GIVEN
	p := Probe{}
	p.graph = newGraph(t)

	metricServerName := "hostFoo"
	metricSoftwareCmdline := "nc -kl 8000"
	metricConnections := "1.2.3.4:80,9.9.9.9:53"
	metricListen := "192.168.1.36:8000,192.168.1.22:8000"
	agentData := fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": 1603890543
    }
  ]
}`, metricConnections, metricListen, metricSoftwareCmdline, metricServerName)

	// hostFoo server
	sendAgentData(t, p, []byte(agentData))
	// hostBar server
	sendAgentData(t, p, []byte(strings.Replace(agentData, metricServerName, "hostBar", 1)))

	// WHEN
	// This should delete all connections in all nodes, as the threshold time is in the future
	p.cleanSoftwareNodes(time.Now().Add(time.Hour))

	// THEN
	swNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeSoftware))
	for _, software := range p.graph.GetNodes(swNodeFilter) {
		softwareTCPConn, err := getTCPConn(software)
		if err != nil {
			t.Errorf("Not able to get TCP connection info")
		}
		assert.Empty(t, softwareTCPConn)

		softwareListenEndpoints, err := getListenEndpoints(software)
		if err != nil {
			t.Errorf("Not able to get TCP listen endpoints")
		}
		assert.Empty(t, softwareListenEndpoints)
	}
}

//
// TestMigrateConnInfoFromOthersToKnownSoftwareNode check if connection info previously sent to 'others' is moved to a known software node if it
// is created after
func TestMigrateConnInfoFromOthersToKnownSoftwareNode(t *testing.T) {
	// GIVEN
	p := Probe{}
	p.graph = newGraph(t)

	metricServerName := "hostFoo"
	metricSoftwareCmdline := "nc -kl 8000"
	metricConnections := "1.2.3.4:80,9.9.9.9:53"
	metricListen := "192.168.1.36:8000,192.168.1.22:8000"
	agentData := fmt.Sprintf(`
{
  "metrics": [
    {
      "fields": {
        "conn": "%s",
        "listen": "%s"
      },
      "name": "procstat_test",
      "tags": {
        "cmdline": "%s",
        "host": "%s",
        "process_name": "nc"
      },
      "timestamp": 1603890543
    }
  ]
}`, metricConnections, metricListen, metricSoftwareCmdline, metricServerName)

	// This should create server "hostFoo" and software "others"
	sendAgentData(t, p, []byte(agentData))

	// WHEN
	// Create a software node for netcat linked to "hostFoo" server
	netcatSoftwareName := "Netcat"
	netcatNode, err := p.graph.NewNode(graph.GenID(), graph.Metadata{
		MetadataNameKey:    netcatSoftwareName,
		MetadataTypeKey:    MetadataTypeSoftware,
		MetadataCmdlineKey: metricSoftwareCmdline,
	})
	if err != nil {
		t.Errorf("Unable to create server %s", netcatSoftwareName)
	}

	serverNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataTypeKey, MetadataTypeServer))
	server := p.graph.GetNodes(serverNodeFilter)[0]
	_, err = p.graph.NewEdge("", server, netcatNode, graph.Metadata{
		MetadataRelationTypeKey: RelationTypeHasSoftware,
	})
	if err != nil {
		t.Errorf("Unable to create edge between server %s and software %s", server, netcatSoftwareName)
	}

	// Send again the process metrics
	sendAgentData(t, p, []byte(agentData))

	// THEN
	// others software should have not connections, neither listeners
	swNodeFilter := graph.NewElementFilter(filters.NewTermStringFilter(MetadataNameKey, OthersSoftwareNode))
	others := p.graph.GetNodes(swNodeFilter)[0]

	othersTCPConn, err := getTCPConn(others)
	if err != nil {
		t.Errorf("Not able to get TCP connection info for others node")
	}
	assert.Empty(t, othersTCPConn)

	othersListenEndpoints, err := getListenEndpoints(others)
	if err != nil {
		t.Errorf("Not able to get TCP listen endpoints for others node")
	}
	assert.Empty(t, othersListenEndpoints)

	// netcat software should have two connections and two listener
	swNodeFilter = graph.NewElementFilter(filters.NewTermStringFilter(MetadataNameKey, netcatSoftwareName))
	netcat := p.graph.GetNodes(swNodeFilter)[0]

	netcatTCPConn, err := getTCPConn(netcat)
	if err != nil {
		t.Errorf("Not able to get TCP connection info for netcat node")
	}
	assert.Len(t, netcatTCPConn, len(strings.Split(metricConnections, ",")))

	netcatListenEndpoints, err := getListenEndpoints(netcat)
	if err != nil {
		t.Errorf("Not able to get TCP listen endpoints for netcat node")
	}
	assert.Len(t, netcatListenEndpoints, len(strings.Split(metricListen, ",")))
}

// TestNotSignalUpdateForKnownNetworkUpdates checks that updateNetworkMetadata does not return true (modificated) if
// the only modification is updating the fields UpdatedAt and Revision
func TestNotSignalUpdateForKnownNetworkUpdates(t *testing.T) {
	newNetworkInfo := generateProcInfoData([]string{"1.1.1.1:80"}, 1e9)
	nodeNetworkInfo := NetworkInfo{}
	var nodeRevisionForceFlush int64 = 100
	p := Probe{
		nodeRevisionForceFlush: nodeRevisionForceFlush,
	}

	// First time updateNetworkMetadata is called, there is no info in nodeNetworkInfo, so it should return true
	assert.Truef(t, p.updateNetworkMetadata(&nodeNetworkInfo, newNetworkInfo, 1), "new network info")

	// This time, its the same network info, just with a new timestamp, the function should not mark is as an update
	newNetworkInfo = generateProcInfoData([]string{"1.1.1.1:80"}, 1e9+1)
	assert.Falsef(t, p.updateNetworkMetadata(&nodeNetworkInfo, newNetworkInfo, 2), "update without new network info")

	// If a new connection is added, it should return true (modification)
	newNetworkInfo = generateProcInfoData([]string{"2.2.2.2:8000"}, 1e9+2)
	assert.Truef(t, p.updateNetworkMetadata(&nodeNetworkInfo, newNetworkInfo, 3), "new network info")

	// After several node modifications the function should return "true" even if it has only
	// updated the UpdatedAt and Revision values.
	// This is to avoid leaving the backend behind too much
	assert.True(t, p.updateNetworkMetadata(&nodeNetworkInfo, newNetworkInfo, nodeRevisionForceFlush), "forced flush iteration %v", nodeRevisionForceFlush)
	assert.True(t, p.updateNetworkMetadata(&nodeNetworkInfo, newNetworkInfo, nodeRevisionForceFlush*2), "forced flush iteration %v", nodeRevisionForceFlush*2)
}

func generateProcInfoData(conn []string, metricTimestamp int64) NetworkInfo {
	ret := NetworkInfo{}
	for _, c := range conn {
		ret[c] = ProcInfo{
			CreatedAt: metricTimestamp,
			UpdatedAt: metricTimestamp,
			Revision:  1,
		}
	}

	return ret
}
