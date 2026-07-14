// Package agentserver implements the rexec.v1.Agent gRPC service on top of the
// enrollment service and the agent's identity.
package agentserver

import (
	"context"
	"runtime"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/inovacc/remote-exec/internal/enroll"
	"github.com/inovacc/remote-exec/internal/execute"
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

// chunkStream is satisfied by both Agent_ExecServer and Agent_DeployServer.
type chunkStream interface {
	Send(*rexecv1.ExecChunk) error
	Context() context.Context
}

// Exec runs a non-destructive command, streaming its output. Gated at
// rex:operator by the authz interceptor.
func (s *Server) Exec(req *rexecv1.ExecRequest, stream rexecv1.Agent_ExecServer) error {
	return runStream(req, stream)
}

// Deploy runs a destructive command, streaming its output. Gated at rex:admin.
// The P4 gate will interpose the agent policy check and NeedsApproval flow here.
func (s *Server) Deploy(req *rexecv1.ExecRequest, stream rexecv1.Agent_DeployServer) error {
	return runStream(req, stream)
}

func runStream(req *rexecv1.ExecRequest, stream chunkStream) error {
	spec := execute.Spec{
		Command:    req.GetCommand(),
		Args:       req.GetArgs(),
		WorkingDir: req.GetWorkingDir(),
		Env:        req.GetEnv(),
	}
	emit := func(c execute.Chunk) error {
		chunk := &rexecv1.ExecChunk{}
		if c.Stderr != nil {
			chunk.Msg = &rexecv1.ExecChunk_Stderr{Stderr: c.Stderr}
		} else {
			chunk.Msg = &rexecv1.ExecChunk_Stdout{Stdout: c.Stdout}
		}
		return stream.Send(chunk)
	}
	code, err := execute.Run(stream.Context(), spec, emit)
	if err != nil {
		return status.Errorf(codes.Internal, "exec: %v", err)
	}
	return stream.Send(&rexecv1.ExecChunk{Msg: &rexecv1.ExecChunk_ExitCode{ExitCode: int32(code)}})
}
