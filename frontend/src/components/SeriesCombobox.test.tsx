import { describe, expect, it, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { SeriesCombobox } from './SeriesCombobox';
import type { SearchResponse } from '@/lib/types';

function renderWithClient(ui: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>);
}

const fetchMock = vi.fn();
const sampleResponse: SearchResponse = {
  language: 'en',
  searchResults: [
    {
      id: 42009,
      title: 'Black Mirror',
      firstAirDate: '2011-12-04',
      poster: 'https://image.tmdb.org/abc.jpg',
      rating: 8.3,
      description: 'A British anthology series.',
    },
    {
      id: 1,
      title: 'Black Sails',
      firstAirDate: '2014-01-25',
      poster: 'https://image.tmdb.org/def.jpg',
      rating: 8.0,
      description: 'A pirate prequel to Treasure Island.',
    },
  ],
};

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

describe('<SeriesCombobox />', () => {
  it('renders a debounced dropdown of search results and commits selection on click', async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify(sampleResponse), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      }),
    );
    const onSelect = vi.fn();

    renderWithClient(<SeriesCombobox onSelect={onSelect} />);

    const input = screen.getByRole('combobox');
    await user.type(input, 'bla');

    await waitFor(() => screen.getByText('Black Mirror'));
    expect(screen.getAllByRole('option')).toHaveLength(2);

    await user.click(screen.getByText('Black Mirror'));
    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({ id: 42009, title: 'Black Mirror' }),
      'en',
    );
  });

  it('Enter commits the highlighted item via arrow keys', async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify(sampleResponse), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      }),
    );
    const onSelect = vi.fn();
    const onSubmit = vi.fn();

    renderWithClient(<SeriesCombobox onSelect={onSelect} onSubmit={onSubmit} />);

    const input = screen.getByRole('combobox');
    await user.type(input, 'bla');
    await waitFor(() => screen.getByText('Black Mirror'));

    await user.keyboard('{ArrowDown}{ArrowDown}{Enter}');
    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({ id: 1, title: 'Black Sails' }),
      'en',
    );
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('bare Enter (no arrow nav) calls onSubmit with the trimmed query', async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify(sampleResponse), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      }),
    );
    const onSelect = vi.fn();
    const onSubmit = vi.fn();

    renderWithClient(<SeriesCombobox onSelect={onSelect} onSubmit={onSubmit} />);

    const input = screen.getByRole('combobox');
    await user.type(input, '  black  ');
    await waitFor(() => screen.getByText('Black Mirror'));

    await user.keyboard('{Enter}');
    expect(onSubmit).toHaveBeenCalledWith('black');
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('does not search for queries shorter than 2 chars', async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify(sampleResponse), { status: 200 }),
    );

    renderWithClient(<SeriesCombobox onSelect={() => {}} />);
    await user.type(screen.getByRole('combobox'), 'a');

    await new Promise((r) => setTimeout(r, 350));
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
