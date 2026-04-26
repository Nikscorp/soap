import { describe, expect, it, beforeEach, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { EpisodesList } from './EpisodesList';
import type { EpisodesResponse, SearchResult } from '@/lib/types';

const series: SearchResult = {
  id: 42009,
  title: 'Black Mirror',
  firstAirDate: '2011-12-04',
  poster: 'https://image.tmdb.org/t/p/w92/abc.jpg',
  rating: 8.3,
  description: '',
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

function jsonResponse(payload: EpisodesResponse) {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { 'content-type': 'application/json' },
  });
}

describe('<EpisodesList />', () => {
  it('renders selected series header and the default-best slice in chronological order', async () => {
    const data: EpisodesResponse = {
      title: 'Black Mirror',
      poster: series.poster,
      defaultBest: 2,
      totalEpisodes: 6,
      episodes: [
        {
          title: 'The National Anthem',
          description: 'A shocking demand from a kidnapper.',
          rating: 7.504,
          number: 1,
          season: 1,
        },
        {
          title: 'Black Museum',
          description: 'A museum of criminological artifacts.',
          rating: 7.858,
          number: 6,
          season: 4,
        },
      ],
    };
    fetchMock.mockResolvedValue(jsonResponse(data));

    renderWithClient(<EpisodesList series={series} language="en" />);
    await waitFor(() => screen.getByText(/Best of/i));
    expect(screen.getByText('S1E1')).toBeInTheDocument();
    expect(screen.getByText('The National Anthem')).toBeInTheDocument();
    expect(screen.getByText('A shocking demand from a kidnapper.')).toBeInTheDocument();
    expect(screen.getByText('S4E6')).toBeInTheDocument();
    expect(screen.getByText('A museum of criminological artifacts.')).toBeInTheDocument();
  });

  it('shows the no-ratings empty state when totalEpisodes is zero', async () => {
    const data: EpisodesResponse = {
      title: 'Black Mirror',
      poster: series.poster,
      defaultBest: 0,
      totalEpisodes: 0,
      episodes: [],
    };
    fetchMock.mockResolvedValue(jsonResponse(data));

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

  it('renders a slider initialized to defaultBest with the right bounds', async () => {
    const data: EpisodesResponse = {
      title: 'Black Mirror',
      poster: series.poster,
      defaultBest: 3,
      totalEpisodes: 6,
      episodes: [
        { title: 'The National Anthem', description: '', rating: 7.504, number: 1, season: 1 },
        { title: 'Fifteen Million Merits', description: '', rating: 7.696, number: 2, season: 1 },
        { title: 'Black Museum', description: '', rating: 7.858, number: 6, season: 4 },
      ],
    };
    fetchMock.mockResolvedValue(jsonResponse(data));

    renderWithClient(<EpisodesList series={series} language="en" />);
    const slider = (await screen.findByRole('slider')) as HTMLInputElement;
    expect(slider).toHaveAttribute('aria-valuenow', '3');
    expect(slider).toHaveAttribute('aria-valuemin', '1');
    expect(slider).toHaveAttribute('aria-valuemax', '6');
  });

  it('dragging the slider down re-slices locally without refetching', async () => {
    const data: EpisodesResponse = {
      title: 'Black Mirror',
      poster: series.poster,
      defaultBest: 3,
      totalEpisodes: 6,
      episodes: [
        { title: 'Lowest', description: '', rating: 6.0, number: 1, season: 1 },
        { title: 'Mid', description: '', rating: 7.5, number: 2, season: 1 },
        { title: 'Best', description: '', rating: 9.5, number: 1, season: 2 },
      ],
    };
    fetchMock.mockResolvedValue(jsonResponse(data));

    renderWithClient(<EpisodesList series={series} language="en" />);
    await screen.findByRole('slider');
    expect(screen.getByText('Lowest')).toBeInTheDocument();
    expect(screen.getByText('Mid')).toBeInTheDocument();
    expect(screen.getByText('Best')).toBeInTheDocument();

    const callsBefore = fetchMock.mock.calls.length;
    const slider = screen.getByRole('slider') as HTMLInputElement;
    fireEvent.change(slider, { target: { value: '1' } });

    await waitFor(() => {
      expect(screen.queryByText('Lowest')).not.toBeInTheDocument();
      expect(screen.queryByText('Mid')).not.toBeInTheDocument();
    });
    expect(screen.getByText('Best')).toBeInTheDocument();
    expect(fetchMock.mock.calls.length).toBe(callsBefore);
  });

  it('only the highest-rated episodes are shown after dragging down, but in chronological order', async () => {
    const data: EpisodesResponse = {
      title: 'Show',
      poster: series.poster,
      defaultBest: 4,
      totalEpisodes: 8,
      episodes: [
        // Returned in chronological order; ratings deliberately scrambled.
        { title: 'A', description: '', rating: 5.0, number: 1, season: 1 },
        { title: 'B', description: '', rating: 9.0, number: 2, season: 1 },
        { title: 'C', description: '', rating: 7.0, number: 1, season: 2 },
        { title: 'D', description: '', rating: 8.0, number: 2, season: 2 },
      ],
    };
    fetchMock.mockResolvedValue(jsonResponse(data));

    renderWithClient(<EpisodesList series={series} language="en" />);
    const slider = (await screen.findByRole('slider')) as HTMLInputElement;

    fireEvent.change(slider, { target: { value: '2' } });

    await waitFor(() => {
      expect(screen.queryByText('A')).not.toBeInTheDocument();
      expect(screen.queryByText('C')).not.toBeInTheDocument();
    });
    const list = screen.getByRole('list');
    const titles = within(list)
      .getAllByRole('listitem')
      .map((li) => li.textContent ?? '');
    // Top 2 by rating are B (9.0) and D (8.0); displayed in (season, number) order.
    expect(titles[0]).toMatch(/S1E2.*B/);
    expect(titles[1]).toMatch(/S2E2.*D/);
  });

  it('dragging the slider above the fetched count refetches with ?limit=N', async () => {
    const initial: EpisodesResponse = {
      title: 'Show',
      poster: series.poster,
      defaultBest: 2,
      totalEpisodes: 4,
      episodes: [
        { title: 'A', description: '', rating: 9.0, number: 1, season: 1 },
        { title: 'B', description: '', rating: 8.0, number: 2, season: 1 },
      ],
    };
    const expanded: EpisodesResponse = {
      ...initial,
      episodes: [
        { title: 'A', description: '', rating: 9.0, number: 1, season: 1 },
        { title: 'B', description: '', rating: 8.0, number: 2, season: 1 },
        { title: 'C', description: '', rating: 7.0, number: 1, season: 2 },
        { title: 'D', description: '', rating: 6.0, number: 2, season: 2 },
      ],
    };
    fetchMock.mockImplementation((input: string | URL) => {
      const url = typeof input === 'string' ? input : input.toString();
      if (url.includes('limit=4')) return Promise.resolve(jsonResponse(expanded));
      return Promise.resolve(jsonResponse(initial));
    });

    renderWithClient(<EpisodesList series={series} language="en" />);
    const slider = (await screen.findByRole('slider')) as HTMLInputElement;
    expect(screen.queryByText('C')).not.toBeInTheDocument();

    fireEvent.change(slider, { target: { value: '4' } });

    await waitFor(() => expect(screen.getByText('C')).toBeInTheDocument());
    expect(screen.getByText('D')).toBeInTheDocument();
    const limitCalls = fetchMock.mock.calls.filter(([input]) => {
      const url = typeof input === 'string' ? input : (input as URL).toString();
      return url.includes('limit=4');
    });
    expect(limitCalls.length).toBeGreaterThanOrEqual(1);
  });
});
