package daemon

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestJSON(t *testing.T) {
	t.Run("marshals request with action only", func(t *testing.T) {
		req := Request{Action: "ping"}
		data, err := json.Marshal(req)
		require.NoError(t, err)

		var decoded Request
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "ping", decoded.Action)
		assert.Nil(t, decoded.Data)
	})

	t.Run("marshals request with data", func(t *testing.T) {
		params := map[string]string{"name": "test-puck"}
		paramsJSON, _ := json.Marshal(params)
		req := Request{Action: "start", Data: paramsJSON}

		data, err := json.Marshal(req)
		require.NoError(t, err)

		var decoded Request
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, "start", decoded.Action)

		var decodedParams map[string]string
		err = json.Unmarshal(decoded.Data, &decodedParams)
		require.NoError(t, err)
		assert.Equal(t, "test-puck", decodedParams["name"])
	})

	t.Run("unmarshals request from JSON string", func(t *testing.T) {
		jsonStr := `{"action":"create","data":{"name":"my-puck","image":"fedora:latest"}}`
		var req Request
		err := json.Unmarshal([]byte(jsonStr), &req)
		require.NoError(t, err)
		assert.Equal(t, "create", req.Action)
		assert.NotNil(t, req.Data)
	})
}

func TestResponseJSON(t *testing.T) {
	t.Run("marshals success response without data", func(t *testing.T) {
		resp := Response{Success: true}
		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var decoded Response
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.True(t, decoded.Success)
		assert.Empty(t, decoded.Error)
	})

	t.Run("marshals success response with data", func(t *testing.T) {
		puck := map[string]interface{}{"name": "test", "status": "running"}
		puckJSON, _ := json.Marshal(puck)
		resp := Response{Success: true, Data: puckJSON}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var decoded Response
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.True(t, decoded.Success)

		var decodedPuck map[string]interface{}
		err = json.Unmarshal(decoded.Data, &decodedPuck)
		require.NoError(t, err)
		assert.Equal(t, "test", decodedPuck["name"])
	})

	t.Run("marshals error response", func(t *testing.T) {
		resp := Response{Success: false, Error: "puck not found"}
		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var decoded Response
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.False(t, decoded.Success)
		assert.Equal(t, "puck not found", decoded.Error)
	})

	t.Run("marshals partial success response", func(t *testing.T) {
		// Used by destroy-all when some pucks fail
		destroyed := []string{"puck1", "puck2"}
		destroyedJSON, _ := json.Marshal(destroyed)
		resp := Response{Success: false, Error: "some pucks failed", Data: destroyedJSON}

		data, err := json.Marshal(resp)
		require.NoError(t, err)

		var decoded Response
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.False(t, decoded.Success)
		assert.Equal(t, "some pucks failed", decoded.Error)

		var decodedDestroyed []string
		err = json.Unmarshal(decoded.Data, &decodedDestroyed)
		require.NoError(t, err)
		assert.Equal(t, []string{"puck1", "puck2"}, decodedDestroyed)
	})
}

func TestRequestActions(t *testing.T) {
	// Test that all expected action names are valid string constants
	actions := []string{
		"create",
		"list",
		"get",
		"start",
		"stop",
		"destroy",
		"destroy-all",
		"snapshot-create",
		"snapshot-restore",
		"snapshot-list",
		"snapshot-delete",
		"ping",
	}

	for _, action := range actions {
		t.Run("action_"+action, func(t *testing.T) {
			req := Request{Action: action}
			data, err := json.Marshal(req)
			require.NoError(t, err)

			var decoded Request
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, action, decoded.Action)
		})
	}
}

// Note: Full handler tests require a mock Manager and Router.
// The handleRequest logic is tested indirectly through client_test.go
// which uses a mock server. Additional integration tests would require
// either dependency injection or running with real Podman.
