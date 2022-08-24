package lazysoap

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
)

func (s *Server) searchHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	query, err := url.QueryUnescape(vars["query"])
	if err != nil {
		log.Printf("[ERROR] Failed to unescape url %s: %v", vars["query"], err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	parsedSeries, err := s.OMDB.SearchInImdb(query)
	if err != nil {
		log.Printf("[ERROR] Failed to get resp from imdb %s: %v", query, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	marshalledResp, err := json.Marshal(parsedSeries)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal response %+v: %v", parsedSeries, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(marshalledResp)
	if err != nil {
		log.Printf("[ERROR] Can't write response: %v", err)
	}
}
