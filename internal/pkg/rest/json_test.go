package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON_Success(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(context.Background(), map[string]any{"a": 1, "b": "x"}, rec)

	res := rec.Result()
	t.Cleanup(func() { _ = res.Body.Close() })

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

	var got map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
	assert.Equal(t, "x", got["b"])
}

func TestWriteJSON_MarshalError(t *testing.T) {
	rec := httptest.NewRecorder()
	// Channels can't be marshalled; this triggers the error branch.
	WriteJSON(context.Background(), make(chan int), rec)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Empty(t, rec.Header().Get("Content-Type"))
}
