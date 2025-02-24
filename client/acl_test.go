package client

import (
	"testing"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestClient_ACL_resolveTokenValue(t *testing.T) {
	ci.Parallel(t)

	s1, _, _, cleanupS1 := testACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.ACLEnabled = true
	})
	defer cleanup()

	// Create a policy / token
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()
	token := mock.ACLToken()
	token.Policies = []string{policy.Name, policy2.Name}
	token2 := mock.ACLToken()
	token2.Type = structs.ACLManagementToken
	token2.Policies = nil
	err := s1.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 100, []*structs.ACLPolicy{policy, policy2})
	assert.Nil(t, err)
	err = s1.State().UpsertACLTokens(structs.MsgTypeTestSetup, 110, []*structs.ACLToken{token, token2})
	assert.Nil(t, err)

	// Test the client resolution
	out0, err := c1.resolveTokenValue("")
	assert.Nil(t, err)
	assert.NotNil(t, out0)
	assert.Equal(t, structs.AnonymousACLToken, out0)

	// Test the client resolution
	out1, err := c1.resolveTokenValue(token.SecretID)
	assert.Nil(t, err)
	assert.NotNil(t, out1)
	assert.Equal(t, token, out1)

	out2, err := c1.resolveTokenValue(token2.SecretID)
	assert.Nil(t, err)
	assert.NotNil(t, out2)
	assert.Equal(t, token2, out2)

	out3, err := c1.resolveTokenValue(token.SecretID)
	assert.Nil(t, err)
	assert.NotNil(t, out3)
	if out1 != out3 {
		t.Fatalf("bad caching")
	}
}

func TestClient_ACL_resolvePolicies(t *testing.T) {
	ci.Parallel(t)

	s1, _, root, cleanupS1 := testACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.ACLEnabled = true
	})
	defer cleanup()

	// Create a policy / token
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()
	token := mock.ACLToken()
	token.Policies = []string{policy.Name, policy2.Name}
	token2 := mock.ACLToken()
	token2.Type = structs.ACLManagementToken
	token2.Policies = nil
	err := s1.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 100, []*structs.ACLPolicy{policy, policy2})
	assert.Nil(t, err)
	err = s1.State().UpsertACLTokens(structs.MsgTypeTestSetup, 110, []*structs.ACLToken{token, token2})
	assert.Nil(t, err)

	// Test the client resolution
	out, err := c1.resolvePolicies(root.SecretID, []string{policy.Name, policy2.Name})
	assert.Nil(t, err)
	assert.Equal(t, 2, len(out))

	// Test caching
	out2, err := c1.resolvePolicies(root.SecretID, []string{policy.Name, policy2.Name})
	assert.Nil(t, err)
	assert.Equal(t, 2, len(out2))

	// Check we get the same objects back (ignore ordering)
	if out[0] != out2[0] && out[0] != out2[1] {
		t.Fatalf("bad caching")
	}
}

func TestClient_ACL_ResolveToken_Disabled(t *testing.T) {
	ci.Parallel(t)

	s1, _, cleanupS1 := testServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanup()

	// Should always get nil when disabled
	aclObj, err := c1.ResolveToken("blah")
	assert.Nil(t, err)
	assert.Nil(t, aclObj)
}

func TestClient_ACL_ResolveToken(t *testing.T) {
	ci.Parallel(t)

	s1, _, _, cleanupS1 := testACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.ACLEnabled = true
	})
	defer cleanup()

	// Create a policy / token
	policy := mock.ACLPolicy()
	policy2 := mock.ACLPolicy()
	token := mock.ACLToken()
	token.Policies = []string{policy.Name, policy2.Name}
	token2 := mock.ACLToken()
	token2.Type = structs.ACLManagementToken
	token2.Policies = nil
	err := s1.State().UpsertACLPolicies(structs.MsgTypeTestSetup, 100, []*structs.ACLPolicy{policy, policy2})
	assert.Nil(t, err)
	err = s1.State().UpsertACLTokens(structs.MsgTypeTestSetup, 110, []*structs.ACLToken{token, token2})
	assert.Nil(t, err)

	// Test the client resolution
	out, err := c1.ResolveToken(token.SecretID)
	assert.Nil(t, err)
	assert.NotNil(t, out)

	// Test caching
	out2, err := c1.ResolveToken(token.SecretID)
	assert.Nil(t, err)
	if out != out2 {
		t.Fatalf("should be cached")
	}

	// Test management token
	out3, err := c1.ResolveToken(token2.SecretID)
	assert.Nil(t, err)
	if acl.ManagementACL != out3 {
		t.Fatalf("should be management")
	}

	// Test bad token
	out4, err := c1.ResolveToken(uuid.Generate())
	assert.Equal(t, structs.ErrTokenNotFound, err)
	assert.Nil(t, out4)
}

func TestClient_ACL_ResolveSecretToken(t *testing.T) {
	ci.Parallel(t)

	s1, _, _, cleanupS1 := testACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
		c.ACLEnabled = true
	})
	defer cleanup()

	token := mock.ACLToken()

	err := s1.State().UpsertACLTokens(structs.MsgTypeTestSetup, 110, []*structs.ACLToken{token})
	assert.Nil(t, err)

	respToken, err := c1.ResolveSecretToken(token.SecretID)
	assert.Nil(t, err)
	if assert.NotNil(t, respToken) {
		assert.NotEmpty(t, respToken.AccessorID)
	}

}
