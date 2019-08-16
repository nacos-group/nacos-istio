package service

import (
	"log"
	"strings"
	"sync"

	"github.com/gogo/protobuf/types"
	"istio.io/api/mcp/v1alpha1"
	"istio.io/api/networking/v1alpha3"
)

// Representation of the endpoints - used to serve EDS and ServiceEntries over MCP and XDS.
//

type Endpoints struct {
	mutex    sync.RWMutex
	seShards map[string]map[string][]*v1alpha3.ServiceEntry
}

var (
	ep = &Endpoints{
		seShards: map[string]map[string][]*v1alpha3.ServiceEntry{},
	}
)

const ServiceEntriesType = "istio/networking/v1alpha3/serviceentries"

func init() {
	resourceHandler["ServiceEntry"] = sePush
	resourceHandler[ServiceEntriesType] = sePush
	resourceHandler["type.googleapis.com/envoy.api.v2.ClusterLoadAssignment"] = edsPush
}

// Called to request push of endpoints in ServiceEntry format
func sePush(s *AdsService, con *Connection, rtype string, res []string) error {
	log.Print("SE request ", rtype, res)

	r := &v1alpha1.Resources{}
	r.Collection = ServiceEntriesType // must match
	for hostname, sh := range ep.seShards {
		res, err := convertServiceEntriesToResource(hostname, sh)
		if err != nil {
			return err
		}
		r.Resources = append(r.Resources, *res)

	}

	return s.Send(con, rtype, r)
}

// Called to request push of ClusterLoadAssignments (EDS) - same information, but in Envoy format
func edsPush(s *AdsService, con *Connection, rtype string, res []string) error {
	// TODO.
	return nil
}

// Called when a new endpoint is added to a shard.
func (fx *AdsService) ServiceEntriesUpdate(shard, hostname string, entry []*v1alpha3.ServiceEntry) error {
	ep.mutex.Lock()
	defer ep.mutex.Unlock()

	sh, f := ep.seShards[hostname]
	if !f {
		sh = map[string][]*v1alpha3.ServiceEntry{}
		ep.seShards[hostname] = sh
	}

	sh[shard] = entry

	log.Println("SEUpdate ", shard, hostname, entry)

	// Typically this is deployed for a single cluster - but may still group in shards.

	// See sink.go - handleResponse.
	r := &v1alpha1.Resources{}

	r.Collection = ServiceEntriesType // must match

	res, err := convertServiceEntriesToResource(hostname, sh)
	if err != nil {
		return err
	}

	r.Resources = []v1alpha1.Resource{*res}
	// The object created by client has resource.Body.TypeUrl, resource.Metadata and Body==Message.

	// TODO: remove the extra caching in coremodel

	fx.SendAll(r)

	return nil
}

// Return all ServiceEntries for a host, as an MCP resource.
func convertServiceEntriesToResource(hostname string, sh map[string][]*v1alpha3.ServiceEntry) (*v1alpha1.Resource, error) {
	// See serviceregistry/external/conversion for the opposite side
	// See galley/pkg/runtime/state
	hostParts := strings.Split(hostname, ".")
	name := hostParts[0]
	var namespace string
	if len(hostParts) == 1 {
		namespace = "consul"
	} else {
		namespace = hostParts[1]
	}

	se := &v1alpha3.ServiceEntry{
		Hosts: []string{hostname},
	}

	for _, serviceEntriesShard := range sh {
		for _, se := range serviceEntriesShard {
			se.Endpoints = append(se.Endpoints, se.Endpoints...)
		}
	}

	seAny, err := types.MarshalAny(se)
	if err != nil {
		return nil, err
	}
	res := v1alpha1.Resource{
		Body: seAny,
		Metadata: &v1alpha1.Metadata{
			Annotations: map[string]string{
				"virtual": "1",
			},
			Name: namespace + "/" + name, // goes to model.Config.Name and Namespace - of course different syntax
		},
	}

	res.Metadata.Version = "1" // model.Config.ResourceVersion
	// Labels and Annotations - for the top service, not used here

	return &res, nil
}

// Called on pod events.
func (fx *AdsService) WorkloadUpdate(id string, labels map[string]string, annotations map[string]string) {
	// update-Running seems to be readiness check ?
	log.Println("PodUpdate ", id, labels, annotations)
}

func (*AdsService) ConfigUpdate(bool) {
	//log.Println("ConfigUpdate")
}

// Updating the internal data structures

// SvcUpdate is called when a service port mapping definition is updated.
// This interface is WIP - labels, annotations and other changes to service may be
// updated to force a EDS and CDS recomputation and incremental push, as it doesn't affect
// LDS/RDS.
func (fx *AdsService) SvcUpdate(shard, hostname string, ports map[string]uint32, rports map[uint32]string) {
	log.Println("ConfigUpdate")
}
