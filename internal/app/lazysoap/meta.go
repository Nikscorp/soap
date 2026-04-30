package lazysoap

import (
	"net/http"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/Nikscorp/soap/internal/pkg/rest"
)

// metaResp is the payload of GET /meta. It is intentionally narrow: anything
// the SPA needs to know about server-wide configuration that doesn't fit on
// a per-request endpoint goes here. Today the only consumer is the footer
// attribution that conditionally renders the IMDb credit when the rating
// source is IMDb.
type metaResp struct {
	RatingsSource string `json:"ratingsSource"`
}

// metaHandler serves GET /meta with cached server metadata. The response is
// allowed to be cached by the browser for a short window because the values
// only change on server restart with a new config.
func (s *Server) metaHandler(w http.ResponseWriter, r *http.Request) {
	source := s.config.RatingsSource
	if source == "" {
		source = "tmdb"
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	rest.WriteJSON(logger.WithAttrs(r.Context(), "ratings_source", source), &metaResp{
		RatingsSource: source,
	}, w)
}
