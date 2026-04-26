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
}

export interface EpisodesResponse {
  episodes: Episode[] | null;
  title: string;
  poster: string;
  defaultBest: number;
  totalEpisodes: number;
}
