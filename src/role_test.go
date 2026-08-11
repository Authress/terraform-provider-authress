package authress

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	authress "github.com/authress/authress-sdk.go"
	"github.com/authress/authress-sdk.go/apis"
	"github.com/authress/authress-sdk.go/models"
	"github.com/authress/authress-sdk.go/providers/jwt"
	TerraformType "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testRoleJSON = `{
	"roleId": "ro_test",
	"name": "Test Role",
	"description": "A test role",
	"permissions": [
		{"action": "read", "allow": true, "grant": false, "delegate": false},
		{"action": "write", "allow": true, "grant": true, "delegate": false}
	]
}`

func newTestClient(server *httptest.Server) *authress.AuthressClient {
	serverURL, _ := url.Parse(server.URL)
	tokenProvider := jwt.NewTokenProvider("test-token")
	settings := authress.AuthressSettings{AuthressApiUrl: serverURL}
	return authress.NewAuthressClient(settings, tokenProvider)
}

func TestMapSdkRoleToTerraform(t *testing.T) {
	roleId := "ro_test"
	sdkRole := &models.Role{
		RoleId: &roleId,
		Name:   "Test Role",
		Permissions: []models.PermissionObject{
			{Action: "read", Allow: true, Grant: false, Delegate: false},
			{Action: "write", Allow: true, Grant: true, Delegate: false},
		},
	}
	sdkRole.SetDescription("A test role")

	result := MapSdkRoleToTerraform(sdkRole)

	assert.Equal(t, "ro_test", result.RoleID.ValueString())
	assert.Equal(t, "ro_test", result.LegacyID.ValueString())
	assert.Equal(t, "Test Role", result.Name.ValueString())
	assert.Equal(t, "A test role", result.Description.ValueString())
	require.Len(t, result.Permissions, 2)

	readPerm := result.Permissions["read"]
	assert.True(t, readPerm.Allow.ValueBool())
	assert.False(t, readPerm.Grant.ValueBool())
	assert.False(t, readPerm.Delegate.ValueBool())

	writePerm := result.Permissions["write"]
	assert.True(t, writePerm.Allow.ValueBool())
	assert.True(t, writePerm.Grant.ValueBool())
	assert.False(t, writePerm.Delegate.ValueBool())
}

func TestMapTerraformRoleToSdk(t *testing.T) {
	terraformRole := &AuthressRoleResource{
		RoleID:      TerraformType.StringValue("ro_test"),
		Name:        TerraformType.StringValue("Test Role"),
		Description: TerraformType.StringValue("A test role"),
		Permissions: map[string]AuthressRolePermissionResource{
			"read":  {Allow: TerraformType.BoolValue(true), Grant: TerraformType.BoolValue(false), Delegate: TerraformType.BoolValue(false)},
			"write": {Allow: TerraformType.BoolValue(true), Grant: TerraformType.BoolValue(true), Delegate: TerraformType.BoolValue(false)},
		},
	}

	result := MapTerraformRoleToSdk(terraformRole)

	assert.Equal(t, "ro_test", result.GetRoleId())
	assert.Equal(t, "Test Role", result.Name)
	assert.Equal(t, "A test role", result.GetDescription())
	require.Len(t, result.Permissions, 2)

	permMap := make(map[string]models.PermissionObject)
	for _, p := range result.Permissions {
		permMap[p.Action] = p
	}

	assert.True(t, permMap["read"].Allow)
	assert.False(t, permMap["read"].Grant)
	assert.True(t, permMap["write"].Allow)
	assert.True(t, permMap["write"].Grant)
}

func TestCreateRole_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/roles":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(testRoleJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	roleId := "ro_test"
	role := &models.Role{
		RoleId:      &roleId,
		Name:        "Test Role",
		Permissions: []models.PermissionObject{{Action: "read", Allow: true}},
	}

	result, resp, err := client.Roles.CreateRole(ctx, role)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "ro_test", result.GetRoleId())
	assert.Equal(t, "Test Role", result.Name)
	assert.Len(t, result.Permissions, 2)
}

func TestGetRole_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/roles/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testRoleJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	result, resp, err := client.Roles.GetRole(ctx, "ro_test")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ro_test", result.GetRoleId())
	assert.Equal(t, "Test Role", result.Name)
}

func TestUpdateRole_Success(t *testing.T) {
	updatedJSON := `{
		"roleId": "ro_test",
		"name": "Updated Role",
		"description": "Updated description",
		"permissions": [
			{"action": "read", "allow": true, "grant": false, "delegate": false},
			{"action": "write", "allow": true, "grant": true, "delegate": true}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/v1/roles/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(updatedJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	roleId := "ro_test"
	role := &models.Role{
		RoleId:      &roleId,
		Name:        "Updated Role",
		Permissions: []models.PermissionObject{{Action: "read", Allow: true}},
	}

	result, resp, err := client.Roles.UpdateRole(ctx, "ro_test", role)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Updated Role", result.Name)
}

func TestDeleteRole_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/v1/roles/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	resp, err := client.Roles.DeleteRole(ctx, "ro_test")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestCreateRole_409_AdoptFlow(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/roles":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"errorCode": "conflict"}`))
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/roles/"):
			callCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testRoleJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	roleId := "ro_test"
	role := &models.Role{
		RoleId:      &roleId,
		Name:        "Test Role",
		Permissions: []models.PermissionObject{{Action: "read", Allow: true}},
	}

	// Create returns 409 (conflict)
	_, _, err := client.Roles.CreateRole(ctx, role)
	require.Error(t, err)

	var clientErr *apis.ClientHttpError
	require.True(t, errors.As(err, &clientErr))
	assert.Equal(t, 409, clientErr.StatusCode())

	// Follow-up GET succeeds (simulating the adopt flow)
	result, _, getErr := client.Roles.GetRole(ctx, "ro_test")
	require.NoError(t, getErr)
	assert.Equal(t, "ro_test", result.GetRoleId())
	assert.Equal(t, 1, callCount)

	// Verify the adopted role maps correctly to Terraform state
	tfRole := MapSdkRoleToTerraform(result)
	assert.Equal(t, "ro_test", tfRole.RoleID.ValueString())
	assert.Equal(t, "Test Role", tfRole.Name.ValueString())
}

func TestGetRole_404_ReturnsClientHttpError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/roles/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errorCode": "not_found"}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	result, _, err := client.Roles.GetRole(ctx, "ro_nonexistent")
	assert.Nil(t, result)
	require.Error(t, err)

	var clientErr *apis.ClientHttpError
	require.True(t, errors.As(err, &clientErr))
	assert.Equal(t, 404, clientErr.StatusCode())
}

func TestCreateRole_RequestBodyIsCorrect(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/roles":
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(testRoleJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	roleId := "ro_test"
	role := &models.Role{
		RoleId: &roleId,
		Name:   "Test Role",
		Permissions: []models.PermissionObject{
			{Action: "read", Allow: true, Grant: false, Delegate: false},
		},
	}
	role.SetDescription("A test role")

	_, _, err := client.Roles.CreateRole(ctx, role)
	require.NoError(t, err)

	assert.Equal(t, "ro_test", receivedBody["roleId"])
	assert.Equal(t, "Test Role", receivedBody["name"])
	assert.Equal(t, "A test role", receivedBody["description"])

	perms := receivedBody["permissions"].([]interface{})
	assert.Len(t, perms, 1)
	firstPerm := perms[0].(map[string]interface{})
	assert.Equal(t, "read", firstPerm["action"])
	assert.Equal(t, true, firstPerm["allow"])
}
