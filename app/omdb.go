package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

type SearchResults struct {
	Results []SearchResult `json:"Search"`
}

type SearchResult struct {
	Title  string `json:"title"`
	ImdbID string `json:"imdbID"`
	Year   string `json:"year"`
	Poster string `json:"poster"`
	Rating string `json:"imdbRating"`
}

type SeriesResult struct {
	Title   string `json:"title"`
	ImdbID  string `json:"imdbID"`
	Year    string `json:"year"`
	Seasons string `json:"totalSeasons"`
	Rating  string `json:"imdbRating"`
	Poster  string `json:"poster"`
}

type Episodes struct {
	Episodes []Episode `json:"Episodes"`
	Title    string    `json:"Title"`
	Poster   string    `json:"Poster"`
}

type Episode struct {
	Title       string  `json:"Title"`
	Rating      string  `json:"imdbRating"`
	Number      string  `json:"Episode"`
	Season      string  `json:"Season"`
	FloatRating float64 `json:"floatRating"`
}

type EpisodeFilterFunc func(Episode) bool

func FilterEpisodes(episodes []Episode, f EpisodeFilterFunc) []Episode {
	res := make([]Episode, 0, len(episodes))
	for _, e := range episodes {
		if f(e) {
			res = append(res, e)
		}
	}
	return res
}

func SearchInImdb(query string) ([]SearchResult, error) {
	customUrl, err := url.Parse(omdbApiUrl)
	if err != nil {
		return nil, err
	}
	parameters := url.Values{}
	parameters.Add("apikey", opts.ApiKey)
	parameters.Add("s", query)
	parameters.Add("type", "series")
	customUrl.RawQuery = parameters.Encode()
	resp, err := http.Get(customUrl.String())
	if err != nil {
		return nil, err
	}
	r := new(SearchResults)
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return nil, err
	}

	return r.Results, nil
}

func GetByImdbID(id string) (*SeriesResult, error) {
	customUrl, err := url.Parse(omdbApiUrl)
	if err != nil {
		return nil, err
	}
	parameters := url.Values{}
	parameters.Add("apikey", opts.ApiKey)
	parameters.Add("i", id)
	parameters.Add("type", "series")
	customUrl.RawQuery = parameters.Encode()
	resp, err := http.Get(customUrl.String())
	if err != nil {
		return nil, err
	}
	r := new(SeriesResult)
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func GetEpisodesBySeason(id string, season int) ([]Episode, error) {
	customUrl, err := url.Parse(omdbApiUrl)
	if err != nil {
		return nil, err
	}
	parameters := url.Values{}
	parameters.Add("apikey", opts.ApiKey)
	parameters.Add("i", id)
	parameters.Add("type", "series")
	parameters.Add("season", strconv.Itoa(season))
	customUrl.RawQuery = parameters.Encode()
	resp, err := http.Get(customUrl.String())
	if err != nil {
		return nil, err
	}
	r := new(Episodes)
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return nil, err
	}

	return r.Episodes, nil
}
