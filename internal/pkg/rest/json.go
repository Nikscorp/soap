package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Nikscorp/soap/internal/pkg/logger"
)

func WriteJSON(ctx context.Context, data interface{}, w http.ResponseWriter) {
	marshalledResp, err := json.Marshal(data)
	if err != nil {
		logger.Error(ctx, "Failed to marshal response", "err", err, "data", fmt.Sprintf("%+v", data))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(marshalledResp)
}
