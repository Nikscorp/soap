import { useState } from 'react';
import { describe, expect, it, beforeEach, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { EpisodesList, type EpisodesListHint } from './EpisodesList';
import type { EpisodesResponse } from '@/lib/types';

const seriesId = 42009;
const language = 'en';
const hint: EpisodesListHint = {
  title: 'Black Mirror',
  poster: 'https://image.tmdb.org/t/p/w92/abc.jpg',
  firstAirDate: '2011-12-04',
};

function renderWithClient(ui: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return {
    client,
    ...render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>),
  };
}

const fetchMock = vi.fn();
const noop = () => undefined;

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

interface RenderProps {
  best?: number | null;
  selectedSeasons?: number[] | null;
  onBestChange?: (next: number | null) => void;
  onSeasonsChange?: (next: number[] | null) => void;
}

function renderList(props: RenderProps = {}) {
  return renderWithClient(
    <EpisodesList
      seriesId={seriesId}
      language={language}
      best={props.best ?? null}
      selectedSeasons={props.selectedSeasons ?? null}
      hint={hint}
      onBestChange={props.onBestChange ?? noop}
      onSeasonsChange={props.onSeasonsChange ?? noop}
    />,
  );
}

describe('<EpisodesList />', () => {
  it('renders selected series header and the default-best slice in chronological order', async () => {
    const data: EpisodesResponse = {
      title: 'Black Mirror',
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 2,
      totalEpisodes: 6,
      availableSeasons: [1, 2, 3, 4],
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

    renderList();
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
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 0,
      totalEpisodes: 0,
      availableSeasons: [],
      episodes: [],
    };
    fetchMock.mockResolvedValue(jsonResponse(data));

    renderList();
    await waitFor(() =>
      expect(
        screen.getByText(/cannot find best episodes because there are no ratings on IMDb/i),
      ).toBeInTheDocument(),
    );
  });

  it('renders the service-unavailable error on backend failure', async () => {
    fetchMock.mockResolvedValue(new Response('boom', { status: 500 }));

    renderList();
    await waitFor(() => expect(screen.getByText(/Service unavailable/i)).toBeInTheDocument());
  });

  it('renders an invalid-filter message with a clear button on 400 with seasons filter', async () => {
    fetchMock.mockResolvedValue(new Response('bad filter', { status: 400 }));

    const onSeasonsChange = vi.fn();
    renderList({ selectedSeasons: [99], onSeasonsChange });
    await waitFor(() =>
      expect(
        screen.getByText(/No episodes found for the selected seasons/i),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByText(/Service unavailable/i)).not.toBeInTheDocument();

    const clearBtn = screen.getByRole('button', { name: /Show all seasons/i });
    fireEvent.click(clearBtn);
    expect(onSeasonsChange).toHaveBeenCalledWith(null);
  });

  it('renders service-unavailable (not invalid-filter) on 400 with no seasons filter', async () => {
    fetchMock.mockResolvedValue(new Response('bad', { status: 400 }));

    renderList({ selectedSeasons: null });
    await waitFor(() => expect(screen.getByText(/Service unavailable/i)).toBeInTheDocument());
    expect(screen.queryByText(/No episodes found/i)).not.toBeInTheDocument();
  });

  it('renders a slider initialized to defaultBest with the right bounds', async () => {
    const data: EpisodesResponse = {
      title: 'Black Mirror',
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 3,
      totalEpisodes: 6,
      availableSeasons: [1, 2, 3, 4],
      episodes: [
        { title: 'The National Anthem', description: '', rating: 7.504, number: 1, season: 1 },
        { title: 'Fifteen Million Merits', description: '', rating: 7.696, number: 2, season: 1 },
        { title: 'Black Museum', description: '', rating: 7.858, number: 6, season: 4 },
      ],
    };
    fetchMock.mockResolvedValue(jsonResponse(data));

    renderList();
    const slider = (await screen.findByRole('slider')) as HTMLInputElement;
    expect(slider).toHaveAttribute('aria-valuenow', '3');
    expect(slider).toHaveAttribute('aria-valuemin', '1');
    expect(slider).toHaveAttribute('aria-valuemax', '6');
  });

  it('initializes the slider from the URL `best` param when provided', async () => {
    const data: EpisodesResponse = {
      title: 'Black Mirror',
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 3,
      totalEpisodes: 6,
      availableSeasons: [1, 2],
      episodes: [
        { title: 'A', description: '', rating: 9.0, number: 1, season: 1 },
        { title: 'B', description: '', rating: 8.0, number: 2, season: 1 },
        { title: 'C', description: '', rating: 7.0, number: 1, season: 2 },
        { title: 'D', description: '', rating: 6.0, number: 2, season: 2 },
        { title: 'E', description: '', rating: 5.0, number: 3, season: 2 },
      ],
    };
    fetchMock.mockResolvedValue(jsonResponse(data));

    renderList({ best: 5 });
    const slider = (await screen.findByRole('slider')) as HTMLInputElement;
    expect(slider).toHaveAttribute('aria-valuenow', '5');
  });

  it('dragging the slider down re-slices locally without refetching', async () => {
    const data: EpisodesResponse = {
      title: 'Black Mirror',
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 3,
      totalEpisodes: 6,
      availableSeasons: [1, 2],
      episodes: [
        { title: 'Lowest', description: '', rating: 6.0, number: 1, season: 1 },
        { title: 'Mid', description: '', rating: 7.5, number: 2, season: 1 },
        { title: 'Best', description: '', rating: 9.5, number: 1, season: 2 },
      ],
    };
    fetchMock.mockResolvedValue(jsonResponse(data));

    renderList();
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
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 4,
      totalEpisodes: 8,
      availableSeasons: [1, 2],
      episodes: [
        { title: 'A', description: '', rating: 5.0, number: 1, season: 1 },
        { title: 'B', description: '', rating: 9.0, number: 2, season: 1 },
        { title: 'C', description: '', rating: 7.0, number: 1, season: 2 },
        { title: 'D', description: '', rating: 8.0, number: 2, season: 2 },
      ],
    };
    fetchMock.mockResolvedValue(jsonResponse(data));

    renderList();
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
    expect(titles[0]).toMatch(/S1E2.*B/);
    expect(titles[1]).toMatch(/S2E2.*D/);
  });

  it('dragging the slider above the fetched count refetches with ?limit=N', async () => {
    const initial: EpisodesResponse = {
      title: 'Show',
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 2,
      totalEpisodes: 4,
      availableSeasons: [1, 2],
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

    renderList();
    const slider = (await screen.findByRole('slider')) as HTMLInputElement;
    expect(screen.queryByText('C')).not.toBeInTheDocument();

    fireEvent.change(slider, { target: { value: '4' } });

    await waitFor(() => expect(screen.getByText('C')).toBeInTheDocument());
    expect(screen.getByText('D')).toBeInTheDocument();
    const limitCalls = fetchMock.mock.calls.filter(([input]) => {
      const url = typeof input === 'string' ? input : (input as URL).toString();
      return url.includes('limit=4');
    });
    expect(limitCalls.length).toBe(1);
  });

  it('reports slider changes via onBestChange (debounced) and omits when value equals defaultBest', async () => {
    vi.useFakeTimers();
    const data: EpisodesResponse = {
      title: 'Show',
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 3,
      totalEpisodes: 4,
      availableSeasons: [1, 2],
      episodes: [
        { title: 'A', description: '', rating: 9.0, number: 1, season: 1 },
        { title: 'B', description: '', rating: 8.0, number: 2, season: 1 },
        { title: 'C', description: '', rating: 7.0, number: 1, season: 2 },
      ],
    };
    fetchMock.mockResolvedValue(jsonResponse(data));
    const onBestChange = vi.fn();

    renderList({ onBestChange });

    // Wait for the slider via fake timers + microtasks.
    await vi.runAllTimersAsync();
    const slider = screen.getByRole('slider') as HTMLInputElement;

    fireEvent.change(slider, { target: { value: '2' } });
    expect(onBestChange).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(400);
    expect(onBestChange).toHaveBeenLastCalledWith(2);

    fireEvent.change(slider, { target: { value: '3' } });
    await vi.advanceTimersByTimeAsync(400);
    expect(onBestChange).toHaveBeenLastCalledWith(null);

    vi.useRealTimers();
  });

  it('renders the SeasonSelector populated from availableSeasons regardless of filter', async () => {
    const data: EpisodesResponse = {
      title: 'Show',
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 1,
      totalEpisodes: 2,
      availableSeasons: [1, 2, 3, 4],
      episodes: [
        { title: 'A', description: '', rating: 9.0, number: 1, season: 1 },
        { title: 'B', description: '', rating: 8.0, number: 2, season: 1 },
      ],
    };
    fetchMock.mockResolvedValue(jsonResponse(data));

    renderList({ selectedSeasons: [1] });
    await screen.findByText(/Best of/i);

    const group = screen.getByRole('group', { name: /filter seasons/i });
    const chipLabels = within(group)
      .getAllByRole('button')
      .map((b) => b.textContent ?? '');
    // "All" + every season in availableSeasons even though only S1 is selected.
    expect(chipLabels).toEqual(['All', 'S1', 'S2', 'S3', 'S4']);

    const allChip = within(group).getByRole('button', { name: 'All' });
    expect(allChip).toHaveAttribute('aria-pressed', 'false');
    const s1 = within(group).getByRole('button', { name: 'S1' });
    expect(s1).toHaveAttribute('aria-pressed', 'true');
    const s2 = within(group).getByRole('button', { name: 'S2' });
    expect(s2).toHaveAttribute('aria-pressed', 'false');
  });

  it('toggling a season triggers a refetch with the new seasons param and resets the slider', async () => {
    const allData: EpisodesResponse = {
      title: 'Show',
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 4,
      totalEpisodes: 8,
      availableSeasons: [1, 2],
      episodes: [
        { title: 'S1E1', description: '', rating: 9.0, number: 1, season: 1 },
        { title: 'S1E2', description: '', rating: 8.5, number: 2, season: 1 },
        { title: 'S1E3', description: '', rating: 8.0, number: 3, season: 1 },
        { title: 'S1E4', description: '', rating: 7.5, number: 4, season: 1 },
        { title: 'S2E1', description: '', rating: 7.0, number: 1, season: 2 },
        { title: 'S2E2', description: '', rating: 6.5, number: 2, season: 2 },
      ],
    };
    const filteredData: EpisodesResponse = {
      title: 'Show',
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 2,
      totalEpisodes: 4,
      availableSeasons: [1, 2],
      episodes: [
        { title: 'S1E1', description: '', rating: 9.0, number: 1, season: 1 },
        { title: 'S1E2', description: '', rating: 8.5, number: 2, season: 1 },
        { title: 'S1E3', description: '', rating: 8.0, number: 3, season: 1 },
        { title: 'S1E4', description: '', rating: 7.5, number: 4, season: 1 },
      ],
    };
    fetchMock.mockImplementation((input: string | URL) => {
      const url = typeof input === 'string' ? input : input.toString();
      if (url.includes('seasons=1')) return Promise.resolve(jsonResponse(filteredData));
      return Promise.resolve(jsonResponse(allData));
    });

    const onSeasonsChange = vi.fn();

    function Harness() {
      const [selected, setSelected] = useState<number[] | null>(null);
      return (
        <EpisodesList
          seriesId={seriesId}
          language={language}
          best={null}
          selectedSeasons={selected}
          hint={hint}
          onBestChange={noop}
          onSeasonsChange={(next) => {
            onSeasonsChange(next);
            setSelected(next);
          }}
        />
      );
    }

    renderWithClient(<Harness />);
    // Initial slider position from `defaultBest` of the unfiltered response.
    const slider = (await screen.findByRole('slider')) as HTMLInputElement;
    await waitFor(() => expect(slider).toHaveAttribute('aria-valuenow', '4'));
    expect(slider).toHaveAttribute('aria-valuemax', '8');

    // Drag the slider away from defaultBest to prove it gets reset on
    // selection change (otherwise sliderValue would be sticky).
    fireEvent.change(slider, { target: { value: '6' } });
    await waitFor(() => expect(slider).toHaveAttribute('aria-valuenow', '6'));

    // Click S1 from all-mode — selects only [1].
    const group = screen.getByRole('group', { name: /filter seasons/i });
    const s1 = within(group).getByRole('button', { name: 'S1' });
    fireEvent.click(s1);

    expect(onSeasonsChange).toHaveBeenCalledTimes(1);
    expect(onSeasonsChange).toHaveBeenCalledWith([1]);

    // The new request should include seasons=1.
    await waitFor(() => {
      const calls = fetchMock.mock.calls.map(([input]) =>
        typeof input === 'string' ? input : (input as URL).toString(),
      );
      expect(calls.some((u) => u.includes('seasons=1'))).toBe(true);
    });

    // Episode list reflects the filtered response (no S2 episodes).
    await waitFor(() => expect(screen.queryByText('S2E1')).not.toBeInTheDocument());

    // Slider resets to the new response's defaultBest (2) and uses the new
    // total (4) as max — the dragged-up "6" must not stick.
    await waitFor(() => {
      const newSlider = screen.getByRole('slider') as HTMLInputElement;
      expect(newSlider).toHaveAttribute('aria-valuenow', '2');
      expect(newSlider).toHaveAttribute('aria-valuemax', '4');
    });
  });

  it('selection change does not re-issue a `limit=` (fetchLimit reset to undefined)', async () => {
    // Initial: backend returns only 2 episodes (cap reached via defaultBest);
    // dragging the slider to 5 triggers a refetch with limit=5 that returns
    // an expanded payload. After that, fetchLimit is sticky at 5 until reset.
    const initialSmall: EpisodesResponse = {
      title: 'Show',
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 2,
      totalEpisodes: 6,
      availableSeasons: [1, 2],
      episodes: [
        { title: 'A', description: '', rating: 9.0, number: 1, season: 1 },
        { title: 'B', description: '', rating: 8.0, number: 2, season: 1 },
      ],
    };
    const expandedAll: EpisodesResponse = {
      title: 'Show',
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 2,
      totalEpisodes: 6,
      availableSeasons: [1, 2],
      episodes: [
        { title: 'A', description: '', rating: 9.0, number: 1, season: 1 },
        { title: 'B', description: '', rating: 8.0, number: 2, season: 1 },
        { title: 'C', description: '', rating: 7.0, number: 1, season: 2 },
        { title: 'D', description: '', rating: 6.5, number: 2, season: 2 },
        { title: 'E', description: '', rating: 6.0, number: 3, season: 2 },
      ],
    };
    const filteredData: EpisodesResponse = {
      title: 'Show',
      poster: hint.poster,
      firstAirDate: hint.firstAirDate,
      defaultBest: 1,
      totalEpisodes: 2,
      availableSeasons: [1, 2],
      episodes: [
        { title: 'A', description: '', rating: 9.0, number: 1, season: 1 },
        { title: 'B', description: '', rating: 8.0, number: 2, season: 1 },
      ],
    };
    fetchMock.mockImplementation((input: string | URL) => {
      const url = typeof input === 'string' ? input : input.toString();
      if (url.includes('seasons=1')) return Promise.resolve(jsonResponse(filteredData));
      if (url.includes('limit=5')) return Promise.resolve(jsonResponse(expandedAll));
      return Promise.resolve(jsonResponse(initialSmall));
    });

    function Harness() {
      const [selected, setSelected] = useState<number[] | null>(null);
      return (
        <EpisodesList
          seriesId={seriesId}
          language={language}
          best={null}
          selectedSeasons={selected}
          hint={hint}
          onBestChange={noop}
          onSeasonsChange={(next) => setSelected(next)}
        />
      );
    }

    renderWithClient(<Harness />);
    const slider = (await screen.findByRole('slider')) as HTMLInputElement;
    // Drag above fetched count so fetchLimit becomes 5 — proves it would
    // have stuck without a reset.
    fireEvent.change(slider, { target: { value: '5' } });
    await waitFor(() => {
      const calls = fetchMock.mock.calls.map(([input]) =>
        typeof input === 'string' ? input : (input as URL).toString(),
      );
      expect(calls.some((u) => u.includes('limit=5'))).toBe(true);
    });

    // Now click S1 → the next request should have seasons=1 but NOT carry
    // the previous fetchLimit=5; otherwise the smaller filtered pool would
    // still be requested with the stale max.
    const group = screen.getByRole('group', { name: /filter seasons/i });
    const s1 = within(group).getByRole('button', { name: 'S1' });
    fireEvent.click(s1);

    await waitFor(() => {
      const seasonsCalls = fetchMock.mock.calls
        .map(([input]) => (typeof input === 'string' ? input : (input as URL).toString()))
        .filter((u) => u.includes('seasons=1'));
      expect(seasonsCalls.length).toBeGreaterThanOrEqual(1);
      expect(seasonsCalls.every((u) => !u.includes('limit='))).toBe(true);
    });
  });
});
