package rest

import (
	"encoding/json"
	"log"
	"net/http"
)

func WriteJSON(data interface{}, w http.ResponseWriter) {
	marshalledResp, err := json.Marshal(data)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal response %+v: %v", data, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(marshalledResp)
}
