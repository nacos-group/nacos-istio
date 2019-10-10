package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/nacos-group/nacos-istio/common"
	"github.com/nacos-group/nacos-istio/nacos"

	ads "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/gogo/protobuf/proto"
	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"istio.io/api/mcp/v1alpha1"
	mcp "istio.io/api/mcp/v1alpha1"
)

var (
	nacks = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xds_nack",
		Help: "Nacks.",
	}, []string{"node", "type"})

	acks = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "xds_ack",
		Help: "Aacks.",
	}, []string{"type"})

	// key is the XDS/MCP type
	resourceHandler = map[string]typeHandler{}
)

// typeHandler is called when a request for a type is first received.
// It should send the list of resources on the connection.
type typeHandler func(s *NacosMcpService, con *Connection, rtype string, res []string) error

//
type NacosMcpService struct {
	grpcServer *grpc.Server

	// mutex used to modify structs, non-blocking code only.
	mutex sync.RWMutex

	// clients reflect active gRPC channels.
	// key is Connection.ConID
	clients map[string]*Connection

	connectionNumber int

	nacosPushService *nacos.MockNacosService
}

type Connection struct {
	mu sync.RWMutex

	// PeerAddr is the address of the client envoy, from network layer
	PeerAddr string

	NodeID string

	// Time of connection, for debugging
	Connect time.Time

	// ConID is the connection identifier, used as a key in the connection table.
	// Currently based on the node name and a counter.
	ConID string

	// doneChannel will be closed when the client is closed.
	doneChannel chan int

	// Metadata key-value pairs extending the Node identifier
	Metadata map[string]string

	// Watched resources for the connection
	Watched map[string][]string

	NonceSent map[string]string

	NonceAcked map[string]string

	// Only one can be set.
	Stream Stream

	active bool

	LastRequestAcked bool

	LastRequestTime int64
}

// NewService initialized MCP servers.
func NewService(addr string, mockParams common.MockParams) *NacosMcpService {

	// default is mocked:
	pushService := &nacos.MockNacosService{
		MockParams: mockParams,
	}

	if mockParams.Mocked {
		pushService = nacos.NewMockNacosService(mockParams)
	}

	nacosMcpService := &NacosMcpService{
		clients: map[string]*Connection{},
		// Use Mock service :
		nacosPushService: pushService,
	}

	nacosMcpService.initGrpcServer()

	mcp.RegisterResourceSourceServer(nacosMcpService.grpcServer, nacosMcpService)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	nacosMcpService.nacosPushService.SubscribeAllServices(func(resources *v1alpha1.Resources, err error) {

		if err != nil {
			log.Println("subscribe error", err)
			return
		}

		if len(nacosMcpService.clients) == 0 {
			return
		}

		for _, con := range nacosMcpService.clients {

			if con.LastRequestAcked == false {
				log.Println("Last request not finished, ignore.")
				continue
			}
			con.LastRequestAcked = false
		}

		nacosMcpService.SendAll(resources)

	})

	go nacosMcpService.grpcServer.Serve(lis)

	return nacosMcpService
}

type Stream interface {
	// can be mcp.RequestResources or v2.DiscoveryRequest
	Send(proto.Message) error
	// mcp.Resources or v2.DiscoveryResponse
	Recv() (proto.Message, error)

	Context() context.Context

	Process(s *NacosMcpService, con *Connection, message proto.Message) error
}

type mcpStream struct {
	stream mcp.ResourceSource_EstablishResourceStreamServer
}

func (mcps *mcpStream) Send(p proto.Message) error {
	if mp, ok := p.(*mcp.Resources); ok {
		return mcps.stream.Send(mp)
	}
	return errors.New("Invalid stream")
}

func (mcps *mcpStream) Recv() (proto.Message, error) {
	p, err := mcps.stream.Recv()

	if err != nil {
		return nil, err
	}

	return p, err
}

func (mcps *mcpStream) Context() context.Context {
	return context.Background()
}

// Compared with ADS:
//  req.Node -> req.SinkNode
//  metadata struct -> Annotations
//  TypeUrl -> Collection
//  no on-demand (Watched)
func (mcps *mcpStream) Process(s *NacosMcpService, con *Connection, msg proto.Message) error {

	req := msg.(*mcp.RequestResources)
	if !con.active {
		var id string
		if req.SinkNode == nil || req.SinkNode.Id == "" {
			log.Println("Missing node id ", req.String())
			id = con.PeerAddr
		} else {
			id = req.SinkNode.Id
		}

		con.mu.Lock()
		con.NodeID = id
		con.Metadata = req.SinkNode.Annotations
		con.ConID = s.connectionID(con.NodeID)
		con.mu.Unlock()

		s.mutex.Lock()
		s.clients[con.ConID] = con
		s.mutex.Unlock()

		con.active = true

		log.Println("activate new connection:", con)
	}

	rtype := req.Collection

	if req.ErrorDetail != nil && req.ErrorDetail.Message != "" {
		nacks.With(prometheus.Labels{"node": con.NodeID, "type": rtype}).Add(1)
		log.Println("NACK: ", con.NodeID, rtype, req.ErrorDetail)
		return nil
	}

	if req.ErrorDetail != nil && req.ErrorDetail.Code == 0 {
		con.mu.Lock()
		con.NonceAcked[rtype] = req.ResponseNonce
		con.mu.Unlock()
		acks.With(prometheus.Labels{"type": rtype}).Add(1)
		log.Println("error", req.ErrorDetail)
		return nil
	}

	if req.ResponseNonce != "" {
		// This shouldn't happen
		con.mu.Lock()
		lastNonce := con.NonceSent[rtype]
		con.mu.Unlock()

		if lastNonce == req.ResponseNonce {

			log.Println("ACK of:", con.LastRequestTime, " used time(microsecond):", time.Now().UnixNano()/1000-con.LastRequestTime, "\n")
			con.LastRequestAcked = true

			acks.With(prometheus.Labels{"type": rtype}).Add(1)
			con.mu.Lock()
			con.NonceAcked[rtype] = req.ResponseNonce
			con.mu.Unlock()
			return nil
		} else {
			// will resent the resource, set the nonce - next response should be ok.
			log.Println("Unmatching nonce ", req.ResponseNonce, lastNonce)
		}
	}

	// Blocking - read will continue
	err := s.push(con, rtype, nil)
	if err != nil {
		// push failed - disconnect
		log.Println("Closing connection ", err)
		return err
	}

	return nil
}

func (s *NacosMcpService) getAllResources() (r *v1alpha1.Resources) {
	return s.nacosPushService.Resources
}

func (s *NacosMcpService) EstablishResourceStream(mcps mcp.ResourceSource_EstablishResourceStreamServer) error {

	log.Println("establish resource stream.....")

	stream := &mcpStream{stream: mcps}

	con := &Connection{
		Stream:           stream,
		NonceSent:        map[string]string{},
		Metadata:         map[string]string{},
		Watched:          map[string][]string{},
		NonceAcked:       map[string]string{},
		doneChannel:      make(chan int, 2),
		LastRequestAcked: true,
	}

	for {
		// Blocking. Separate go-routines may use the stream to push.
		req, err := stream.Recv()
		if err != nil {
			if status.Code(err) == codes.Canceled || err == io.EOF {
				log.Println("ADS: %q %s terminated %v", con.PeerAddr, con.ConID, err)
				// remove this connection:
				delete(s.clients, con.ConID)
				return nil
			}
			log.Println("ADS: %q %s terminated with errors %v", con.PeerAddr, con.ConID, err)
			return err
		}
		err = stream.Process(s, con, req)
		if err != nil {
			return err
		}
	}

}

// Push a single resource type on the connection. This is blocking.
func (s *NacosMcpService) push(con *Connection, rtype string, res []string) error {
	h, f := resourceHandler[rtype]
	log.Println("push", rtype, f)
	if !f {
		log.Println("Resource not found ", rtype)
		r := &v1alpha1.Resources{}
		r.Collection = rtype
		_ = s.Send(con, rtype, r)
		return nil
	}
	return h(s, con, rtype, res)
}

// IncrementalAggregatedResources is not implemented.
func (s *NacosMcpService) DeltaAggregatedResources(stream ads.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

// Callbacks from the lower layer

func (s *NacosMcpService) initGrpcServer() {
	grpcOptions := s.grpcServerOptions()
	s.grpcServer = grpc.NewServer(grpcOptions...)

}

func (s *NacosMcpService) grpcServerOptions() []grpc.ServerOption {
	interceptors := []grpc.UnaryServerInterceptor{
		// setup server prometheus monitoring (as final interceptor in chain)
		grpcprometheus.UnaryServerInterceptor,
	}

	grpcprometheus.EnableHandlingTimeHistogram()

	// Temp setting, default should be enough for most supported environments. Can be used for testing
	// envoy with lower values.
	var maxStreams int
	if maxStreams == 0 {
		maxStreams = 100000
	}

	grpcOptions := []grpc.ServerOption{
		grpc.UnaryInterceptor(middleware.ChainUnaryServer(interceptors...)),
		grpc.MaxConcurrentStreams(uint32(maxStreams)),
	}

	return grpcOptions
}

func (fx *NacosMcpService) SendAll(r *v1alpha1.Resources) {

	//log.Println("current clients", fx.clients)
	for _, con := range fx.clients {
		con.LastRequestTime = time.Now().UnixNano() / 1000
		log.Println("sending resources", len(r.Resources), con.LastRequestTime, con.ConID)
		r.Nonce = fmt.Sprintf("%v", time.Now())
		con.NonceSent[r.Collection] = r.Nonce
		con.Stream.Send(r)
	}

}

func (fx *NacosMcpService) Send(con *Connection, rtype string, r *v1alpha1.Resources) error {
	r.Nonce = fmt.Sprintf("%v", time.Now())
	con.NonceSent[rtype] = r.Nonce

	//log.Println("sending resources", r)

	return con.Stream.Send(r)
}

func (s *NacosMcpService) connectionID(node string) string {
	s.mutex.Lock()
	s.connectionNumber++
	c := s.connectionNumber
	s.mutex.Unlock()
	return node + "-" + strconv.Itoa(int(c))
}
