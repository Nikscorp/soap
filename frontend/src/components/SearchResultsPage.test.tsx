import { describe, expect, it, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { SearchResultsPage } from './SearchResultsPage';
import type { SearchResponse } from '@/lib/types';

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

function jsonResponse(payload: SearchResponse) {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { 'content-type': 'application/json' },
  });
}

describe('<SearchResultsPage />', () => {
  it('renders cards with title, year, rating, and description', async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({
        language: 'en',
        searchResults: [
          {
            id: 42009,
            title: 'Black Mirror',
            firstAirDate: '2011-12-04',
            poster: 'https://image.tmdb.org/abc.jpg',
            rating: 8.3,
            description: 'A British anthology series about modern technology.',
          },
        ],
      }),
    );

    renderWithClient(<SearchResultsPage q="black mirror" onSelect={vi.fn()} />);

    await waitFor(() => screen.getByText('Black Mirror'));
    expect(screen.getByText('(2011)')).toBeInTheDocument();
    expect(screen.getByLabelText(/Rating 8\.3/)).toBeInTheDocument();
    expect(
      screen.getByText('A British anthology series about modern technology.'),
    ).toBeInTheDocument();
  });

  it('renders the empty state when the backend returns no results', async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({ language: 'en', searchResults: [] }),
    );

    renderWithClient(<SearchResultsPage q="nothing" onSelect={vi.fn()} />);
    await waitFor(() =>
      expect(screen.getByText(/No series match this query/i)).toBeInTheDocument(),
    );
  });

  it('renders the service-unavailable error on backend failure', async () => {
    fetchMock.mockResolvedValue(new Response('boom', { status: 500 }));

    renderWithClient(<SearchResultsPage q="lost" onSelect={vi.fn()} />);
    await waitFor(() =>
      expect(screen.getByText(/Service unavailable/i)).toBeInTheDocument(),
    );
  });

  it('clicking a card invokes onSelect with the result and the response language', async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValue(
      jsonResponse({
        language: 'ru',
        searchResults: [
          {
            id: 1,
            title: 'Лост',
            firstAirDate: '2004-09-22',
            poster: '/img/lost.jpg',
            rating: 9.1,
            description: 'Survivors on a mysterious island.',
          },
        ],
      }),
    );
    const onSelect = vi.fn();

    renderWithClient(<SearchResultsPage q="лост" onSelect={onSelect} />);

    await waitFor(() => screen.getByText('Лост'));
    await user.click(screen.getByRole('button', { name: /Лост/ }));

    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({ id: 1, title: 'Лост' }),
      'ru',
    );
  });

  it('renders a fallback when description is empty', async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({
        language: 'en',
        searchResults: [
          {
            id: 9,
            title: 'No Overview',
            firstAirDate: '',
            poster: '',
            rating: 0,
            description: '',
          },
        ],
      }),
    );

    renderWithClient(<SearchResultsPage q="no overview" onSelect={vi.fn()} />);
    await waitFor(() => screen.getByText('No Overview'));
    expect(screen.getByText(/No description available/i)).toBeInTheDocument();
  });
});
