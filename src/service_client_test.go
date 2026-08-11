package authress

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/authress/authress-sdk.go/apis"
	"github.com/authress/authress-sdk.go/models"
	TerraformType "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testServiceClientJSON = `{
	"clientId": "sc_test_client",
	"name": "Test Service Client",
	"createdTime": "2024-01-15T12:00:00Z",
	"tags": {"env": "test"},
	"options": {
		"grantUserPermissionsAccess": true,
		"grantTokenGeneration": false
	},
	"verificationKeys": [
		{"keyId": "key-001", "publicKey": "MCowBQYDK2VwAyEA"}
	]
}`

var testAccessKeyResponseJSON = `{
	"keyId": "key-002",
	"publicKey": "MCowBQYDK2VwAyEA"
}`

func TestCreateServiceClient_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/clients":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(testServiceClientJSON))
		case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/v1/clients/") && strings.HasSuffix(r.URL.Path, "/access-keys"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(testAccessKeyResponseJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	sdkClient := &models.Client{
		ClientId: "sc_test_client",
	}
	sdkClient.SetName("Test Service Client")

	result, resp, err := client.ServiceClients.CreateClient(ctx, sdkClient)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "sc_test_client", result.ClientId)
	assert.Equal(t, "Test Service Client", result.GetName())

	// Verify access key creation works
	accessKeyBody := &models.ClientAccessKey{}
	accessKeyBody.SetPublicKey("MCowBQYDK2VwAyEA")

	keyResult, keyResp, keyErr := client.ServiceClients.RequestAccessKey(ctx, "sc_test_client", accessKeyBody)
	require.NoError(t, keyErr)
	assert.Equal(t, http.StatusCreated, keyResp.StatusCode)
	assert.Equal(t, "key-002", keyResult.GetKeyId())
}

func TestGetServiceClient_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/clients/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testServiceClientJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	result, resp, err := client.ServiceClients.GetClient(ctx, "sc_test_client")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "sc_test_client", result.ClientId)
	assert.Equal(t, "Test Service Client", result.GetName())
	require.True(t, result.HasVerificationKeys())
	keys := result.GetVerificationKeys()
	require.Len(t, keys, 1)
	assert.Equal(t, "key-001", keys[0].GetKeyId())
	assert.Equal(t, "MCowBQYDK2VwAyEA", keys[0].GetPublicKey())
}

func TestGetServiceClient_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/clients/"):
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

	result, _, err := client.ServiceClients.GetClient(ctx, "sc_nonexistent")
	assert.Nil(t, result)
	require.Error(t, err)

	var clientErr *apis.ClientHttpError
	require.True(t, errors.As(err, &clientErr))
	assert.Equal(t, 404, clientErr.StatusCode())
}

func TestDeleteServiceClient_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/v1/clients/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	resp, err := client.ServiceClients.DeleteClient(ctx, "sc_test_client")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestCreateServiceClient_409_AdoptFlow(t *testing.T) {
	getCallCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/clients":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"errorCode": "conflict"}`))
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/clients/"):
			getCallCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testServiceClientJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	sdkClient := &models.Client{
		ClientId: "sc_test_client",
	}
	sdkClient.SetName("Test Service Client")

	// Create returns 409 (conflict)
	_, _, err := client.ServiceClients.CreateClient(ctx, sdkClient)
	require.Error(t, err)

	var clientErr *apis.ClientHttpError
	require.True(t, errors.As(err, &clientErr))
	assert.Equal(t, 409, clientErr.StatusCode())

	// Follow-up GET succeeds (simulating the adopt flow)
	result, _, getErr := client.ServiceClients.GetClient(ctx, "sc_test_client")
	require.NoError(t, getErr)
	assert.Equal(t, "sc_test_client", result.ClientId)
	assert.Equal(t, 1, getCallCount)

	// Verify the adopted client maps correctly to Terraform state
	tfClient := mapSdkServiceClientToTerraform(result)
	assert.Equal(t, "sc_test_client", tfClient.ClientId.ValueString())
	assert.Equal(t, "Test Service Client", tfClient.Name.ValueString())
	require.NotNil(t, tfClient.Options)
	assert.True(t, tfClient.Options.GrantUserPermissionsAccess.ValueBool())
	assert.False(t, tfClient.Options.GrantTokenGeneration.ValueBool())
}

func TestMapSdkServiceClientToTerraform(t *testing.T) {
	sdkClient := &models.Client{
		ClientId: "sc_test_client",
	}
	sdkClient.SetName("Test Service Client")
	sdkClient.SetTags(map[string]string{"env": "test", "team": "platform"})

	opts := models.NewClientOptions()
	opts.SetGrantUserPermissionsAccess(true)
	opts.SetGrantTokenGeneration(false)
	sdkClient.SetOptions(*opts)

	key1 := models.NewClientAccessKey()
	key1.SetKeyId("key-001")
	key1.SetPublicKey("MCowBQYDK2VwAyEA")
	sdkClient.SetVerificationKeys([]models.ClientAccessKey{*key1})

	result := mapSdkServiceClientToTerraform(sdkClient)

	assert.Equal(t, "sc_test_client", result.ClientId.ValueString())
	assert.Equal(t, "Test Service Client", result.Name.ValueString())

	// Tags
	require.Len(t, result.Tags, 2)
	assert.Equal(t, TerraformType.StringValue("test"), result.Tags["env"])
	assert.Equal(t, TerraformType.StringValue("platform"), result.Tags["team"])

	// Options
	require.NotNil(t, result.Options)
	assert.True(t, result.Options.GrantUserPermissionsAccess.ValueBool())
	assert.False(t, result.Options.GrantTokenGeneration.ValueBool())

	// Access keys (from verificationKeys)
	require.Len(t, result.AccessKeys, 1)
	assert.Equal(t, "key-001", result.AccessKeys[0].KeyId.ValueString())
	assert.Equal(t, "MCowBQYDK2VwAyEA", result.AccessKeys[0].PublicKey.ValueString())
}
