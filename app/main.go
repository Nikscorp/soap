package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	cache "github.com/victorspringer/http-cache"
	"github.com/victorspringer/http-cache/adapter/redis"
)

const staticPath = "/static"
const staticURI = "/"
const ioTimeoutSec = 15
const idleTimeoutSec = 15
const estimatedEpisodesPerSeasonCnt = 20
const omdbAPIURL = "https://omdbapi.com/"

var opts struct {
	Address     string `long:"listen-address" short:"l" default:"0.0.0.0:8080" description:"listen address of http server"`
	RedisAddr   string `long:"redis-addr" short:"r" default:"redis:6379" description:"redis connection address"`
	RedisPasswd string `long:"redis-passwd" short:"p" description:"redis password"`
	APIKey      string `long:"api-key" short:"k" description:"OMDB API key"`
}

func idHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	resp, err := GetByImdbID(id)
	if err != nil {
		log.Printf("[ERROR] Failed to get series by imdb id %s: %v", id, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	seasonsCnt, err := strconv.Atoi(resp.Seasons)
	if err != nil {
		log.Printf("[ERROR] Failed to Parse SeasonsCnt %s: %v", resp.Seasons, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	respEpisodes := make([]Episode, 0, seasonsCnt*estimatedEpisodesPerSeasonCnt)
	var sumRating float64
	var episodesCount int
	for i := 1; i <= seasonsCnt; i++ {
		episodes, err := GetEpisodesBySeason(id, i)
		if err != nil {
			log.Printf("[ERROR] Failed to season %d by imdb id %s: %v", i, id, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		for _, e := range episodes {
			if e.Rating == "N/A" {
				continue
			}
			rating, err := strconv.ParseFloat(e.Rating, 64)
			if err != nil {
				log.Printf("[ERROR] Failed to parse rating %s", e.Rating)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			e.FloatRating = rating
			e.Season = strconv.Itoa(i)
			respEpisodes = append(respEpisodes, e)
			sumRating += rating
			episodesCount++
		}
	}

	avgRating := sumRating / float64(episodesCount)
	log.Printf("[INFO] Avg Rating for id %s is %v", id, avgRating)

	respEpisodes = FilterEpisodes(respEpisodes, func(e Episode) bool {
		return e.FloatRating >= avgRating
	})

	fullRespEpisodes := Episodes{Episodes: respEpisodes, Title: resp.Title, Poster: resp.Poster}
	marshalledResp, err := json.Marshal(fullRespEpisodes)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal response %+v: %v", respEpisodes, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(marshalledResp)
	if err != nil {
		log.Printf("[ERROR] Can't write response: %v", err)
	}
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	query, err := url.QueryUnescape(vars["query"])
	if err != nil {
		log.Printf("[ERROR] Failed to unescape url %s: %v", vars["query"], err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	parsedSeries, err := SearchInImdb(query)
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

func newRouter() *mux.Router {
	ringOpt := &redis.RingOptions{
		Addrs: map[string]string{
			"server": opts.RedisAddr,
		},
		Password: opts.RedisPasswd,
	}
	cacheClient, err := cache.NewClient(
		cache.ClientWithAdapter(redis.NewAdapter(ringOpt)),
		cache.ClientWithTTL(60*time.Minute),
	)

	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	r := mux.NewRouter()
	idHandlerWithMiddlewares := cacheClient.Middleware(http.HandlerFunc(idHandler))
	idHandlerWithMiddlewares = handlers.LoggingHandler(log.Writer(), idHandlerWithMiddlewares)
	r.Handle("/id/{id}", idHandlerWithMiddlewares).Methods("GET", "POST")

	searchHandlerWithMiddlewares := cacheClient.Middleware(http.HandlerFunc(searchHandler))
	searchHandlerWithMiddlewares = handlers.LoggingHandler(log.Writer(), searchHandlerWithMiddlewares)
	r.Handle("/search/{query}", searchHandlerWithMiddlewares).Methods("GET", "POST")
	addFileServer(r)
	return r
}

func main() {
	parseOpts(&opts)
	log.Printf("[INFO] Opts parsed successfully: %+v", opts)

	r := newRouter()
	r2 := handlers.CORS(handlers.AllowedOrigins([]string{"*"}))(r)
	srv := &http.Server{
		Addr:         opts.Address,
		WriteTimeout: time.Second * ioTimeoutSec,
		ReadTimeout:  time.Second * ioTimeoutSec,
		IdleTimeout:  time.Second * idleTimeoutSec,
		Handler:      handlers.RecoveryHandler()(r2),
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	shutDowner := func() {
		sig := <-sigChan
		log.Printf("[WARN] Received signal %v, shutting down the server", sig)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			log.Printf("[ERROR] Failed to shut down the server: %v", err)
			os.Exit(1)
		}
		log.Printf("[INFO] Successfully shutted down the server")
	}
	go shutDowner()
	log.Printf("[INFO] Start to listen http requests")
	err := srv.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Fatalf("[FATAL] ListenAndServe failed with error: %v", err)
	}
	log.Printf("[INFO] Gracefully shutted down")
}
