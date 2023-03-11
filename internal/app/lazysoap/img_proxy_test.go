package lazysoap

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nikscorp/soap/internal/app/lazysoap/mocks"
	"github.com/Nikscorp/soap/internal/pkg/rest"
	"github.com/stretchr/testify/require"
)

// RoundTripFunc ...
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip ...
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(fn),
	}
}

func NewServerWithImgClient(t *testing.T) *ServerM {
	tvMetaClientMock := mocks.NewTvMetaClientMock(t)
	srv := &Server{
		address: "",
		tvMeta:  tvMetaClientMock,
		metrics: rest.NewMetrics([]string{"id", "search", "img"}),
		imgClient: NewTestClient(func(req *http.Request) *http.Response {
			switch path := req.URL.String(); path {
			case "https://image.tmdb.org/t/p/w92/i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg binary data")),
					Header:     make(http.Header),
				}
			case "https://image.tmdb.org/t/p/w92/not_found.jpg":
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(bytes.NewBufferString("some html data")),
					Header:     make(http.Header),
				}
			case "https://image.tmdb.org/t/p/w92/internal_error.jpg":
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(bytes.NewBufferString("some html data")),
					Header:     make(http.Header),
				}
			case "https://image.tmdb.org/t/p/w92/nil_body.jpg":
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       nil,
					Header:     make(http.Header),
				}
			case "https://image.tmdb.org/t/p/w92/transport_error.jpg":
				return nil
			default:
				require.FailNow(t, "invalid path %s", path)
				return nil
			}
		}),
	}
	ts := httptest.NewServer(srv.newRouter())

	return &ServerM{
		server:           ts,
		tvMetaClientMock: tvMetaClientMock,
	}
}

func TestCommon(t *testing.T) {
	srv := NewServerWithImgClient(t)
	defer srv.server.Close()

	tests := []struct {
		name           string
		reqURL         string
		wantStatusCode int
		wantBody       string
	}{
		{
			name:           "positive case",
			reqURL:         "/img/i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg",
			wantStatusCode: http.StatusOK,
			wantBody:       "i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg binary data",
		},
		{
			name:           "not found",
			reqURL:         "/img/not_found.jpg",
			wantStatusCode: http.StatusNotFound,
			wantBody:       "",
		},
		{
			name:           "internal error",
			reqURL:         "/img/internal_error.jpg",
			wantStatusCode: http.StatusInternalServerError,
			wantBody:       "",
		},
		{
			name:           "transport error",
			reqURL:         "/img/transport_error.jpg",
			wantStatusCode: http.StatusInternalServerError,
			wantBody:       "",
		},
		{
			name:           "nil body",
			reqURL:         "/img/nil_body.jpg",
			wantStatusCode: http.StatusInternalServerError,
			wantBody:       "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := http.Get(srv.server.URL + test.reqURL)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			require.Equal(t, test.wantStatusCode, resp.StatusCode)
			require.Equal(t, test.wantBody, string(body))
		})
	}
}
