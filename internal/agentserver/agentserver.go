// Package agentserver implements the rexec.v1.Agent gRPC service on top of the
// enrollment service and the agent's identity.
package agentserver

import (
	"context"
	"runtime"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/inovacc/remote-exec/internal/enroll"
	"github.com/inovacc/remote-exec/internal/execute"
	"github.com/inovacc/remote-exec/internal/policy"
	rexecv1 "github.com/inovacc/remote-exec/internal/proto/rexec/v1"
)

// approvalTTL bounds how long a "needs approval" grant stays valid.
const approvalTTL = 5 * time.Minute

// Server implements rexecv1.AgentServer.
type Server struct {
	rexecv1.UnimplementedAgentServer

	enroller    *enroll.Service
	agentID     string
	fingerprint string
	hostname    string
	version     string
	policy      policy.Policy
	grants      *policy.Grants
}

// New builds an Agent server. fingerprint is the agent's own server-cert
// fingerprint (what controllers pin); pol + grants drive the destructive-op gate
// on Deploy.
func New(enroller *enroll.Service, agentID, fingerprint, hostname, version string, pol policy.Policy, grants *policy.Grants) *Server {
	return &Server{
		enroller:    enroller,
		agentID:     agentID,
		fingerprint: fingerprint,
		hostname:    hostname,
		version:     version,
		policy:      pol,
		grants:      grants,
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

// Deploy runs a destructive command, streaming its output. The authz interceptor
// has already confirmed rex:admin; here the agent's own policy decides. When the
// request carries a valid approval id, it runs; otherwise policy says allow (run),
// deny (reject), or ask (stream a NeedsApproval and stop until the controller
// re-invokes with the id).
func (s *Server) Deploy(req *rexecv1.ExecRequest, stream rexecv1.Agent_DeployServer) error {
	command := req.GetCommand()

	if id := req.GetApprovalId(); id != "" {
		if err := s.grants.Consume(id, command); err != nil {
			return status.Errorf(codes.PermissionDenied, "approval: %v", err)
		}
		return runStream(req, stream)
	}

	switch s.policy.Evaluate(command) {
	case policy.DecisionAllow:
		return runStream(req, stream)
	case policy.DecisionAsk:
		id, err := s.grants.Issue(command, approvalTTL)
		if err != nil {
			return status.Errorf(codes.Internal, "approval: %v", err)
		}
		op := strings.TrimSpace(command + " " + strings.Join(req.GetArgs(), " "))
		return stream.Send(&rexecv1.ExecChunk{Msg: &rexecv1.ExecChunk_NeedsApproval{
			NeedsApproval: &rexecv1.ApprovalRequest{
				Operation:  op,
				Reason:     "agent policy requires approval for destructive operations",
				ApprovalId: id,
			},
		}})
	default:
		return status.Errorf(codes.PermissionDenied, "agent policy denies destructive command %q", command)
	}
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
