package tvmeta

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePosterSize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "w92 passes through", input: "w92", want: "w92"},
		{name: "w154 passes through", input: "w154", want: "w154"},
		{name: "w185 passes through", input: "w185", want: "w185"},
		{name: "w342 passes through", input: "w342", want: "w342"},
		{name: "w500 passes through", input: "w500", want: "w500"},
		{name: "w780 passes through", input: "w780", want: "w780"},
		{name: "disallowed falls back to default", input: "original", want: "w92"},
		{name: "empty falls back to default", input: "", want: "w92"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePosterSize(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

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
