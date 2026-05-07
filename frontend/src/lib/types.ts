// Wire types matching internal/app/lazysoap/{search,by_id}.go.

export interface SearchResult {
  id: number;
  title: string;
  firstAirDate: string;
  poster: string;
  rating: number;
  description: string;
}

export interface SearchResponse {
  searchResults: SearchResult[] | null;
  language: string;
}

export interface Episode {
  title: string;
  description: string;
  rating: number;
  number: number;
  season: number;
  still?: string;
}

export interface EpisodesResponse {
  episodes: Episode[] | null;
  title: string;
  poster: string;
  firstAirDate: string;
  description?: string;
  defaultBest: number;
  totalEpisodes: number;
  availableSeasons: number[];
}

export interface FeaturedSeries {
  id: number;
  title: string;
  firstAirDate: string;
  poster: string;
}

export interface FeaturedResponse {
  series: FeaturedSeries[] | null;
  language: string;
}

// Server-wide config metadata exposed by GET /meta. Currently only carries
// the rating source, which the footer reads to decide whether to render the
// IMDb attribution required by IMDb's data usage policy.
export interface MetaResponse {
  ratingsSource: 'tmdb' | 'imdb' | string;
}
