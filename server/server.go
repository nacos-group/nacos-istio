package server

import (
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	mcp "istio.io/api/mcp/v1alpha1"
	"istio.io/istio/pkg/mcp/rate"
)

type AuthChecker interface {
	Check(authInfo credentials.AuthInfo) error
}

// Server implements the server for the MCP source service. The server is the source of configuration and sends
// configuration to the client.
type Server struct {
	authCheck   AuthChecker
	rateLimiter rate.Limit
	src         *Source
	metadata    metadata.MD
}

var _ mcp.ResourceSourceServer = &Server{}

// ServerOptions contains sink server specific options
type ServerOptions struct {
	AuthChecker AuthChecker
	RateLimiter rate.Limit
	Metadata    metadata.MD
}

// NewServer creates a new instance of a MCP source server.
func NewServer(srcOptions *Options, serverOptions *ServerOptions) *Server {
	s := &Server{
		src:         New(srcOptions),
		authCheck:   serverOptions.AuthChecker,
		rateLimiter: serverOptions.RateLimiter,
		metadata:    serverOptions.Metadata,
	}
	return s
}

// EstablishResourceStream implements the ResourceSourceServer interface.
func (s *Server) EstablishResourceStream(stream mcp.ResourceSource_EstablishResourceStreamServer) error {
	if s.rateLimiter != nil {
		if err := s.rateLimiter.Wait(stream.Context()); err != nil {
			return err
		}

	}
	var authInfo credentials.AuthInfo
	if peerInfo, ok := peer.FromContext(stream.Context()); ok {
		authInfo = peerInfo.AuthInfo
	} else {
		scope.Warnf("No peer info found on the incoming stream.")
	}

	if err := s.authCheck.Check(authInfo); err != nil {
		return status.Errorf(codes.Unauthenticated, "Authentication failure: %v", err)
	}

	if err := stream.SendHeader(s.metadata); err != nil {
		return err
	}
	err := s.src.ProcessStream(stream)
	code := status.Code(err)
	if code == codes.OK || code == codes.Canceled || err == io.EOF {
		return nil
	}
	return err
}
