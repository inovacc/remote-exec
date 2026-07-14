// Package agentserver implements the rexec.v1.Agent gRPC service on top of the
// enrollment service and the agent's identity.
package agentserver

import (
	"context"
	"runtime"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/inovacc/remote-exec/internal/enroll"
	rexecv1 "github.com/inovacc/remote-exec/internal/proto/rexec/v1"
)

// Server implements rexecv1.AgentServer.
type Server struct {
	rexecv1.UnimplementedAgentServer

	enroller    *enroll.Service
	agentID     string
	fingerprint string
	hostname    string
	version     string
}

// New builds an Agent server. fingerprint is the agent's own server-cert
// fingerprint (what controllers pin).
func New(enroller *enroll.Service, agentID, fingerprint, hostname, version string) *Server {
	return &Server{
		enroller:    enroller,
		agentID:     agentID,
		fingerprint: fingerprint,
		hostname:    hostname,
		version:     version,
	}
}

// Enroll validates the join token and signs the controller's client CSR.
func (s *Server) Enroll(_ context.Context, req *rexecv1.EnrollRequest) (*rexecv1.EnrollResponse, error) {
	res, err := s.enroller.Enroll(req.GetCsrPem(), req.GetToken())
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	return &rexecv1.EnrollResponse{
		ClientCertPem: res.ClientCertPEM,
		CaPem:         res.CAPEM,
		AgentId:       res.AgentID,
		Fingerprint:   res.Fingerprint,
	}, nil
}

// Identity returns the agent's stable id and server-cert fingerprint.
func (s *Server) Identity(_ context.Context, _ *rexecv1.IdentityRequest) (*rexecv1.IdentityResponse, error) {
	return &rexecv1.IdentityResponse{AgentId: s.agentID, Fingerprint: s.fingerprint}, nil
}

// Info returns host/os/arch/version.
func (s *Server) Info(_ context.Context, _ *rexecv1.InfoRequest) (*rexecv1.InfoResponse, error) {
	return &rexecv1.InfoResponse{
		Os:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Hostname: s.hostname,
		Version:  s.version,
	}, nil
}
