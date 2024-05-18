package kafka

import (
	"context"
	"strconv"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

const (
	usernamePrefix = "testkafkarole"
	roleName       = "testkafkarole"
	testTTL        = int64(120)
	testMaxTTL     = int64(3600)
)

// TestUserRole uses a mock backend to check
// role create, read, update, and delete.
func TestUserRole(t *testing.T) {
	b, s := getTestBackend(t)

	t.Run("List All Roles", func(t *testing.T) {
		for i := 1; i <= 10; i++ {
			_, err := testTokenRoleCreate(t, b, s,
				roleName+strconv.Itoa(i),
				map[string]interface{}{
					"username_prefix":   usernamePrefix,
					"scram_sha_version": scramSHAVersion,
					"ttl":               testTTL,
					"max_ttl":           testMaxTTL,
				})
			require.NoError(t, err)
		}

		resp, err := testTokenRoleList(t, b, s)
		require.NoError(t, err)
		require.Len(t, resp.Data["keys"].([]string), 10)
	})

	t.Run("Create User Role", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.Nil(t, err)
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Read User Role", func(t *testing.T) {
		resp, err := testTokenRoleRead(t, b, s)

		require.Nil(t, err)
		require.Nil(t, resp.Error())
		require.NotNil(t, resp)
		require.Equal(t, resp.Data["username_prefix"], usernamePrefix)
		require.Equal(t, resp.Data["scram_sha_version"], scramSHAVersion)
		require.Equal(t, resp.Data["resource_acls"], "")
	})

	t.Run("Update User Role", func(t *testing.T) {
		resp, err := testTokenRoleUpdate(t, b, s, map[string]interface{}{
			"scram_sha_version": SCRAMSHA256,
			"ttl":               "1m",
			"max_ttl":           "5h",
		})

		require.Nil(t, err)
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Re-read User Role", func(t *testing.T) {
		resp, err := testTokenRoleRead(t, b, s)

		require.Nil(t, err)
		require.Nil(t, resp.Error())
		require.NotNil(t, resp)
		require.Equal(t, resp.Data["username_prefix"], usernamePrefix)
		require.Equal(t, resp.Data["scram_sha_version"], SCRAMSHA256)
		require.Equal(t, resp.Data["resource_acls"], "")
	})

	t.Run("Delete User Role", func(t *testing.T) {
		_, err := testTokenRoleDelete(t, b, s)

		require.NoError(t, err)
	})

	t.Run("Create User Role With Wrong ACLs - Empty config", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     "",
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.EqualError(t, err, "resource_acls cannot be empty")
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Create User Role With Wrong ACLs - Bad JSON", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     "[{},]",
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.EqualError(t, err, "unable to parse resource_acls: invalid character ']' looking for beginning of value")
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Create User Role With Wrong ACLs - Empty Resource ACL", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     "[{}]",
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.EqualError(t, err, "unable to parse resource_acls: received unknown resource type")
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Create User Role With Wrong ACLs - Unknown Resource Type", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     `[{"ResourceType": "Toppic"}]`,
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.EqualError(t, err, "unable to parse resource_acls: no acl resource with name toppic")
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Create User Role With Wrong ACLs - Empty Resource Name", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     `[{"ResourceType": "Topic", "ResourceName": ""}]`,
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.EqualError(t, err, "unable to parse resource_acls: received empty resource name")
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Create User Role With Wrong ACLs - Empty Resource Pattern", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     `[{"ResourceType": "Topic", "ResourceName": "my-topic", "ResourcePatternType": ""}]`,
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.EqualError(t, err, "unable to parse resource_acls: no acl resource pattern with name ")
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Create User Role With Wrong ACLs - Unknown Resource Pattern", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     `[{"ResourceType": "Topic", "ResourceName": "my-topic", "ResourcePatternType": "pre"}]`,
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.EqualError(t, err, "unable to parse resource_acls: no acl resource pattern with name pre")
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Create User Role With Wrong ACLs - Empty ACL Host", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     `[{"ResourceType": "Topic", "ResourceName": "my-topic", "ResourcePatternType": "literal", "Acls": [{"Host": ""}]}]`,
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.EqualError(t, err, "unable to parse resource_acls: received empty host in resource ACL")
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Create User Role With Wrong ACLs - Empty ACL Operation", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     `[{"ResourceType": "Topic", "ResourceName": "my-topic", "ResourcePatternType": "literal", "Acls": [{"Host": "*", "Operation": ""}]}]`,
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.EqualError(t, err, "unable to parse resource_acls: no acl operation with name ")
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Create User Role With Wrong ACLs - Unknown ACL Operation", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     `[{"ResourceType": "Topic", "ResourceName": "my-topic", "ResourcePatternType": "literal", "Acls": [{"Host": "*", "Operation": "superoperation"}]}]`,
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.EqualError(t, err, "unable to parse resource_acls: no acl operation with name superoperation")
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Create User Role With Wrong ACLs - Empty ACL Permission Type", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     `[{"ResourceType": "Topic", "ResourceName": "my-topic", "ResourcePatternType": "literal", "Acls": [{"Host": "*", "Operation": "all", "PermissionType": ""}]}]`,
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.EqualError(t, err, "unable to parse resource_acls: no acl permission with name ")
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Create User Role With Wrong ACLs - Unknown ACL Permission Type", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     `[{"ResourceType": "Topic", "ResourceName": "my-topic", "ResourcePatternType": "literal", "Acls": [{"Host": "*", "Operation": "all", "PermissionType": "custompermissiontype"}]}]`,
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.EqualError(t, err, "unable to parse resource_acls: no acl permission with name custompermissiontype")
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Create User Role With ACLs", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"username_prefix":   usernamePrefix,
			"scram_sha_version": scramSHAVersion,
			"resource_acls":     `[{"ResourceType": "Topic", "ResourceName": "my-topic", "ResourcePatternType": "literal", "Acls": [{"Host": "*", "Operation": "all", "PermissionType": "allow"}]}]`,
			"ttl":               testTTL,
			"max_ttl":           testMaxTTL,
		})

		require.Nil(t, err)
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Read User Role With ACLs", func(t *testing.T) {
		resp, err := testTokenRoleRead(t, b, s)

		require.Nil(t, err)
		require.Nil(t, resp.Error())
		require.NotNil(t, resp)
		require.Equal(t, resp.Data["username_prefix"], usernamePrefix)
		require.Equal(t, resp.Data["scram_sha_version"], scramSHAVersion)
		require.Equal(t, resp.Data["resource_acls"], `[{"ResourceType": "Topic", "ResourceName": "my-topic", "ResourcePatternType": "literal", "Acls": [{"Host": "*", "Operation": "all", "PermissionType": "allow"}]}]`)
	})

	t.Run("Delete User Role With ACLs", func(t *testing.T) {
		_, err := testTokenRoleDelete(t, b, s)

		require.NoError(t, err)
	})
}

// Utility function to create a role while, returning any response (including errors)
func testTokenRoleCreate(t *testing.T, b *kafkaBackend, s logical.Storage, name string, d map[string]interface{}) (*logical.Response, error) {
	t.Helper()
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "role/" + name,
		Data:      d,
		Storage:   s,
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Utility function to update a role while, returning any response (including errors)
func testTokenRoleUpdate(t *testing.T, b *kafkaBackend, s logical.Storage, d map[string]interface{}) (*logical.Response, error) {
	t.Helper()
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "role/" + roleName,
		Data:      d,
		Storage:   s,
	})
	if err != nil {
		return nil, err
	}

	if resp != nil && resp.IsError() {
		t.Fatal(resp.Error())
	}
	return resp, nil
}

// Utility function to read a role and return any errors
func testTokenRoleRead(t *testing.T, b *kafkaBackend, s logical.Storage) (*logical.Response, error) {
	t.Helper()
	return b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "role/" + roleName,
		Storage:   s,
	})
}

// Utility function to list roles and return any errors
func testTokenRoleList(t *testing.T, b *kafkaBackend, s logical.Storage) (*logical.Response, error) {
	t.Helper()
	return b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ListOperation,
		Path:      "role/",
		Storage:   s,
	})
}

// Utility function to delete a role and return any errors
func testTokenRoleDelete(t *testing.T, b *kafkaBackend, s logical.Storage) (*logical.Response, error) {
	t.Helper()
	return b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      "role/" + roleName,
		Storage:   s,
	})
}
