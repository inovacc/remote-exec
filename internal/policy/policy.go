// Package policy is the agent-side half of the destructive-op gate. After the
// authz interceptor confirms the caller holds rex:admin, the agent consults its
// local policy to decide whether a destructive command may run outright, must be
// denied, or requires live human approval. This is defense in depth: the
// cryptographic role says "may request", the policy says "may run here".
package policy

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Mode is the agent's stance on destructive commands.
type Mode string

const (
	ModeDeny  Mode = "deny"  // no destructive command runs (safe default)
	ModeAllow Mode = "allow" // destructive commands run (optionally allow-list-restricted)
	ModeAsk   Mode = "ask"   // destructive commands need live approval unless pre-approved
)

// Policy is loaded from the agent's policy.yaml.
type Policy struct {
	Destructive Mode     `yaml:"destructive"` // default deny
	Allow       []string `yaml:"allow"`       // in allow mode: the only commands permitted; in ask mode: pre-approved
	Deny        []string `yaml:"deny"`        // always denied (wins over everything)
}

// Decision is the outcome of evaluating a destructive command.
type Decision int

const (
	DecisionDeny Decision = iota
	DecisionAllow
	DecisionAsk
)

// Default returns the safe default policy: deny all destructive commands.
func Default() Policy { return Policy{Destructive: ModeDeny} }

// Load reads a policy from path. A missing file yields the safe default.
func Load(path string) (Policy, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Policy{}, fmt.Errorf("policy: read: %w", err)
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Policy{}, fmt.Errorf("policy: decode: %w", err)
	}
	if p.Destructive == "" {
		p.Destructive = ModeDeny
	}
	return p, nil
}

// Evaluate decides how a destructive command should be handled.
func (p Policy) Evaluate(command string) Decision {
	if contains(p.Deny, command) {
		return DecisionDeny
	}
	switch p.Destructive {
	case ModeAllow:
		if len(p.Allow) > 0 && !contains(p.Allow, command) {
			return DecisionDeny
		}
		return DecisionAllow
	case ModeAsk:
		if contains(p.Allow, command) {
			return DecisionAllow // pre-approved
		}
		return DecisionAsk
	default: // ModeDeny or unknown
		return DecisionDeny
	}
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
