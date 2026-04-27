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

func TestGetURLByPosterPathWithSize(t *testing.T) {
	tests := []struct {
		name       string
		posterPath string
		size       string
		want       string
	}{
		{
			name:       "allowed size is honored",
			posterPath: "i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg",
			size:       "w342",
			want:       "https://image.tmdb.org/t/p/w342/i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg",
		},
		{
			name:       "disallowed size falls back to default",
			posterPath: "i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg",
			size:       "original",
			want:       "https://image.tmdb.org/t/p/w92/i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg",
		},
		{
			name:       "empty size falls back to default",
			posterPath: "i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg",
			size:       "",
			want:       "https://image.tmdb.org/t/p/w92/i8NA7TqNgnXuAtDeQOF5baX0jI6.jpg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetURLByPosterPathWithSize(tt.posterPath, tt.size)
			require.Equal(t, tt.want, got)
		})
	}
}
