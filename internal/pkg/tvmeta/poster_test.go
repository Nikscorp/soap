package tvmeta

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetURLByPosterPath(t *testing.T) {
	tests := []struct {
		name       string
		posterPath string
		want       string
	}{
		{
			name:       "common case",
			posterPath: "i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg",
			want:       "https://image.tmdb.org/t/p/w92/i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg",
		},
		{
			name:       "empty path",
			posterPath: ".jpg",
			want:       "https://image.tmdb.org/t/p/w92/.jpg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetURLByPosterPath(tt.posterPath)
			require.Equal(t, tt.want, got)
		})
	}
}
