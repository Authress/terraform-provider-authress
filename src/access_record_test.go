package authress

import (
	"context"
	"encoding/json"
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

var testAccessRecordJSON = `{
	"recordId": "sc_test123",
	"name": "Service Client Owner Access",
	"users": [{"userId": "sc_test123"}],
	"statements": [
		{
			"roles": ["Authress:Owner"],
			"resources": [{"resourceUri": "*"}]
		}
	]
}`

func TestMapSdkAccessRecordToTerraform(t *testing.T) {
	recordId := "sc_test123"
	sdkRecord := &models.AccessRecord{
		Name: "Service Client Owner Access",
		Users: []models.User{
			{UserId: "sc_test123"},
		},
		Statements: []models.Statement{
			{
				Roles:     []string{"Authress:Owner"},
				Resources: []models.Resource{{ResourceUri: "*"}},
			},
		},
	}
	sdkRecord.SetRecordId(recordId)

	result := mapSdkAccessRecordToTerraform(sdkRecord)

	assert.Equal(t, "sc_test123", result.RecordId.ValueString())
	assert.Equal(t, "Service Client Owner Access", result.Name.ValueString())
	require.Len(t, result.Users, 1)
	assert.Equal(t, "sc_test123", result.Users[0].UserId.ValueString())
	require.Len(t, result.Statements, 1)
	require.Len(t, result.Statements[0].Roles, 1)
	assert.Equal(t, "Authress:Owner", result.Statements[0].Roles[0].ValueString())
	require.Len(t, result.Statements[0].Resources, 1)
	assert.Equal(t, "*", result.Statements[0].Resources[0].ResourceUri.ValueString())
}

func TestMapTerraformAccessRecordToSdk(t *testing.T) {
	tf := &AuthressAccessRecordResource{
		RecordId: TerraformType.StringValue("sc_test123"),
		Name:     TerraformType.StringValue("Service Client Owner Access"),
		Users: []AccessRecordUserResource{
			{UserId: TerraformType.StringValue("sc_test123")},
		},
		Statements: []AccessRecordStatementResource{
			{
				Roles: []TerraformType.String{TerraformType.StringValue("Authress:Owner")},
				Resources: []AccessRecordResourceResource{
					{ResourceUri: TerraformType.StringValue("*")},
				},
			},
		},
	}

	result := mapTerraformAccessRecordToSdk(tf)

	assert.Equal(t, "sc_test123", result.GetRecordId())
	assert.Equal(t, "Service Client Owner Access", result.Name)
	require.Len(t, result.GetUsers(), 1)
	assert.Equal(t, "sc_test123", result.GetUsers()[0].UserId)
	require.Len(t, result.GetStatements(), 1)
	assert.Equal(t, []string{"Authress:Owner"}, result.GetStatements()[0].Roles)
	require.Len(t, result.GetStatements()[0].Resources, 1)
	assert.Equal(t, "*", result.GetStatements()[0].Resources[0].ResourceUri)
}

func TestCreateAccessRecord_PutSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/v1/records/"):
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	record := models.AccessRecord{
		Name:       "Test Record",
		Users:      []models.User{{UserId: "sc_test123"}},
		Statements: []models.Statement{{Roles: []string{"Authress:Owner"}, Resources: []models.Resource{{ResourceUri: "*"}}}},
	}

	resp, err := client.AccessRecords.UpdateRecord(ctx, "sc_test123").AccessRecord(record).Execute()
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGetAccessRecord_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/records/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testAccessRecordJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	result, resp, err := client.AccessRecords.GetRecord(ctx, "sc_test123").Execute()
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "sc_test123", result.GetRecordId())
	assert.Equal(t, "Service Client Owner Access", result.GetName())
	require.Len(t, result.GetUsers(), 1)
	require.Len(t, result.GetStatements(), 1)
}

func TestDeleteAccessRecord_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/v1/records/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	resp, err := client.AccessRecords.DeleteRecord(ctx, "sc_test123").Execute()
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestGetAccessRecord_404_ReturnsClientHttpError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/records/"):
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

	result, _, err := client.AccessRecords.GetRecord(ctx, "rec_nonexistent").Execute()
	assert.Nil(t, result)
	require.Error(t, err)

	var clientErr *apis.ClientHttpError
	require.True(t, errors.As(err, &clientErr))
	assert.Equal(t, 404, clientErr.StatusCode())
}

func TestCreateAccessRecord_409_AdoptFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/v1/records/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"errorCode": "conflict"}`))
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/records/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testAccessRecordJSON))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	record := models.AccessRecord{
		Name:       "Service Client Owner Access",
		Users:      []models.User{{UserId: "sc_test123"}},
		Statements: []models.Statement{{Roles: []string{"Authress:Owner"}, Resources: []models.Resource{{ResourceUri: "*"}}}},
	}

	// PUT returns 409
	_, err := client.AccessRecords.UpdateRecord(ctx, "sc_test123").AccessRecord(record).Execute()
	require.Error(t, err)

	var clientErr *apis.ClientHttpError
	require.True(t, errors.As(err, &clientErr))
	assert.Equal(t, 409, clientErr.StatusCode())

	// GET succeeds (simulating the adopt flow)
	result, _, getErr := client.AccessRecords.GetRecord(ctx, "sc_test123").Execute()
	require.NoError(t, getErr)
	assert.Equal(t, "sc_test123", result.GetRecordId())

	// Verify adoption maps correctly
	tfRecord := mapSdkAccessRecordToTerraform(result)
	assert.Equal(t, "sc_test123", tfRecord.RecordId.ValueString())
	assert.Equal(t, "Service Client Owner Access", tfRecord.Name.ValueString())
}

func TestCreateAccessRecord_RequestBodyIsCorrect(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/v1/records/"):
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(server)
	ctx := context.Background()

	record := models.AccessRecord{
		Name:  "Test Record",
		Users: []models.User{{UserId: "sc_abc"}},
		Statements: []models.Statement{
			{
				Roles:     []string{"Authress:Owner"},
				Resources: []models.Resource{{ResourceUri: "*"}},
			},
		},
	}
	record.SetRecordId("sc_abc")

	_, err := client.AccessRecords.UpdateRecord(ctx, "sc_abc").AccessRecord(record).Execute()
	require.NoError(t, err)

	assert.Equal(t, "sc_abc", receivedBody["recordId"])
	assert.Equal(t, "Test Record", receivedBody["name"])

	users := receivedBody["users"].([]interface{})
	require.Len(t, users, 1)
	assert.Equal(t, "sc_abc", users[0].(map[string]interface{})["userId"])

	statements := receivedBody["statements"].([]interface{})
	require.Len(t, statements, 1)
	stmt := statements[0].(map[string]interface{})
	roles := stmt["roles"].([]interface{})
	assert.Equal(t, "Authress:Owner", roles[0])
	resources := stmt["resources"].([]interface{})
	assert.Equal(t, "*", resources[0].(map[string]interface{})["resourceUri"])
}

func TestCollectAccessRecordMismatches_Matching(t *testing.T) {
	planned := &AuthressAccessRecordResource{
		RecordId: TerraformType.StringValue("sc_test"),
		Name:     TerraformType.StringValue("Test"),
		Users:    []AccessRecordUserResource{{UserId: TerraformType.StringValue("sc_test")}},
		Statements: []AccessRecordStatementResource{
			{
				Roles:     []TerraformType.String{TerraformType.StringValue("Authress:Owner")},
				Resources: []AccessRecordResourceResource{{ResourceUri: TerraformType.StringValue("*")}},
			},
		},
	}

	existing := &models.AccessRecord{
		Name:  "Test",
		Users: []models.User{{UserId: "sc_test"}},
		Statements: []models.Statement{
			{Roles: []string{"Authress:Owner"}, Resources: []models.Resource{{ResourceUri: "*"}}},
		},
	}
	existingId := "sc_test"
	existing.RecordId = &existingId

	mismatches := collectAccessRecordMismatches(planned, existing)
	assert.Nil(t, mismatches)
}

func TestCollectAccessRecordMismatches_Differing(t *testing.T) {
	planned := &AuthressAccessRecordResource{
		RecordId: TerraformType.StringValue("sc_test"),
		Name:     TerraformType.StringValue("Different Name"),
		Users:    []AccessRecordUserResource{{UserId: TerraformType.StringValue("sc_test")}},
		Statements: []AccessRecordStatementResource{
			{
				Roles:     []TerraformType.String{TerraformType.StringValue("Authress:Owner")},
				Resources: []AccessRecordResourceResource{{ResourceUri: TerraformType.StringValue("*")}},
			},
		},
	}

	existing := &models.AccessRecord{
		Name:  "Original Name",
		Users: []models.User{{UserId: "sc_test"}},
		Statements: []models.Statement{
			{Roles: []string{"Authress:Owner"}, Resources: []models.Resource{{ResourceUri: "*"}}},
		},
	}
	existingId := "sc_test"
	existing.RecordId = &existingId

	mismatches := collectAccessRecordMismatches(planned, existing)
	require.NotNil(t, mismatches)
	assert.Len(t, mismatches, 1)
	assert.Equal(t, "name", mismatches[0].Field)
}
