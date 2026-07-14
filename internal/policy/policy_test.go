package policy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEvaluate(t *testing.T) {
	cases := []struct {
		name string
		pol  Policy
		cmd  string
		want Decision
	}{
		{"default is deny", Default(), "kubectl", DecisionDeny},
		{"deny mode", Policy{Destructive: ModeDeny}, "kubectl", DecisionDeny},
		{"allow mode, no list", Policy{Destructive: ModeAllow}, "kubectl", DecisionAllow},
		{"allow mode, in list", Policy{Destructive: ModeAllow, Allow: []string{"kubectl"}}, "kubectl", DecisionAllow},
		{"allow mode, not in list", Policy{Destructive: ModeAllow, Allow: []string{"helm"}}, "kubectl", DecisionDeny},
		{"ask mode", Policy{Destructive: ModeAsk}, "kubectl", DecisionAsk},
		{"ask mode, pre-approved", Policy{Destructive: ModeAsk, Allow: []string{"kubectl"}}, "kubectl", DecisionAllow},
		{"deny list wins over allow", Policy{Destructive: ModeAllow, Deny: []string{"rm"}}, "rm", DecisionDeny},
		{"deny list wins over ask-preapproved", Policy{Destructive: ModeAsk, Allow: []string{"rm"}, Deny: []string{"rm"}}, "rm", DecisionDeny},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.pol.Evaluate(tc.cmd))
		})
	}
}

func TestLoad_MissingIsSafeDefault(t *testing.T) {
	p, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	require.NoError(t, err)
	require.Equal(t, ModeDeny, p.Destructive)
	require.Equal(t, DecisionDeny, p.Evaluate("anything"))
}

func TestLoad_ParsesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.yaml")
	require.NoError(t, os.WriteFile(path, []byte("destructive: ask\nallow: [helm]\ndeny: [rm]\n"), 0o600))

	p, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, ModeAsk, p.Destructive)
	require.Equal(t, DecisionAllow, p.Evaluate("helm"))
	require.Equal(t, DecisionAsk, p.Evaluate("kubectl"))
	require.Equal(t, DecisionDeny, p.Evaluate("rm"))
}

func TestGrants_IssueConsumeSingleUse(t *testing.T) {
	g := NewGrants()
	id, err := g.Issue("kubectl", time.Hour)
	require.NoError(t, err)

	require.NoError(t, g.Consume(id, "kubectl"))
	require.ErrorIs(t, g.Consume(id, "kubectl"), ErrNoGrant, "single-use")
}

func TestGrants_CommandMismatch(t *testing.T) {
	g := NewGrants()
	id, err := g.Issue("kubectl", time.Hour)
	require.NoError(t, err)
	require.ErrorIs(t, g.Consume(id, "helm"), ErrCommandMismatch)
}

func TestGrants_Expired(t *testing.T) {
	g := NewGrants()
	base := time.Now()
	g.now = func() time.Time { return base }
	id, err := g.Issue("kubectl", time.Minute)
	require.NoError(t, err)

	g.now = func() time.Time { return base.Add(2 * time.Minute) }
	require.ErrorIs(t, g.Consume(id, "kubectl"), ErrGrantExpired)
	require.ErrorIs(t, g.Consume(id, "kubectl"), ErrNoGrant, "expired grant is removed")
}
