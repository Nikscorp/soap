import { describe, expect, it, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { EpisodesList } from './EpisodesList';
import type { EpisodesResponse, SearchResult } from '@/lib/types';

const series: SearchResult = {
  id: 42009,
  title: 'Black Mirror',
  firstAirDate: '2011-12-04',
  poster: 'https://image.tmdb.org/t/p/w92/abc.jpg',
  rating: 8.3,
};

function renderWithClient(ui: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

describe('<EpisodesList />', () => {
  it('renders selected series header and its episodes', async () => {
    const data: EpisodesResponse = {
      title: 'Black Mirror',
      poster: series.poster,
      episodes: [
        { title: 'The National Anthem', rating: 7.504, number: 1, season: 1 },
        { title: 'Black Museum', rating: 7.858, number: 6, season: 4 },
      ],
    };
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify(data), { status: 200, headers: { 'content-type': 'application/json' } }),
    );

    renderWithClient(<EpisodesList series={series} language="en" />);
    await waitFor(() => screen.getByText(/Best of/i));
    expect(screen.getByText('S1E1')).toBeInTheDocument();
    expect(screen.getByText('The National Anthem')).toBeInTheDocument();
    expect(screen.getByText('S4E6')).toBeInTheDocument();
  });

  it('shows the no-ratings empty state when episodes is empty', async () => {
    const data: EpisodesResponse = {
      title: 'Black Mirror',
      poster: series.poster,
      episodes: [],
    };
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify(data), { status: 200, headers: { 'content-type': 'application/json' } }),
    );

    renderWithClient(<EpisodesList series={series} language="en" />);
    await waitFor(() =>
      expect(
        screen.getByText(/cannot find best episodes because there are no ratings on IMDb/i),
      ).toBeInTheDocument(),
    );
  });

  it('renders the service-unavailable error on backend failure', async () => {
    fetchMock.mockResolvedValue(new Response('boom', { status: 500 }));

    renderWithClient(<EpisodesList series={series} language="en" />);
    await waitFor(() => expect(screen.getByText(/Service unavailable/i)).toBeInTheDocument());
  });
});
