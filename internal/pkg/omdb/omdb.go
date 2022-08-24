package omdb

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

const (
	omdbAPIURL = "https://omdbapi.com/"
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

type OMDB struct {
	APIKey string
}

func (o *OMDB) FilterEpisodes(episodes []Episode, f EpisodeFilterFunc) []Episode {
	res := make([]Episode, 0, len(episodes))
	for _, e := range episodes {
		if f(e) {
			res = append(res, e)
		}
	}
	return res
}

func (o *OMDB) SearchInImdb(query string) ([]SearchResult, error) {
	customURL, err := url.Parse(omdbAPIURL)
	if err != nil {
		return nil, err
	}
	parameters := url.Values{}
	parameters.Add("apikey", o.APIKey)
	parameters.Add("s", query)
	parameters.Add("type", "series")
	customURL.RawQuery = parameters.Encode()

	resp, err := http.Get(customURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	r := new(SearchResults)
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return nil, err
	}

	return r.Results, nil
}

func (o *OMDB) GetByImdbID(id string) (*SeriesResult, error) {
	customURL, err := url.Parse(omdbAPIURL)
	if err != nil {
		return nil, err
	}
	parameters := url.Values{}
	parameters.Add("apikey", o.APIKey)
	parameters.Add("i", id)
	parameters.Add("type", "series")
	customURL.RawQuery = parameters.Encode()
	resp, err := http.Get(customURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	r := new(SeriesResult)
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (o *OMDB) GetEpisodesBySeason(id string, season int) ([]Episode, error) {
	customURL, err := url.Parse(omdbAPIURL)
	if err != nil {
		return nil, err
	}
	parameters := url.Values{}
	parameters.Add("apikey", o.APIKey)
	parameters.Add("i", id)
	parameters.Add("type", "series")
	parameters.Add("season", strconv.Itoa(season))
	customURL.RawQuery = parameters.Encode()

	resp, err := http.Get(customURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	r := new(Episodes)
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return nil, err
	}

	return r.Episodes, nil
}
