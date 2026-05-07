import { describe, expect, it, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { FeaturedSeries } from './FeaturedSeries';
import type { FeaturedResponse } from '@/lib/types';

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

function jsonResponse(payload: FeaturedResponse) {
  return new Response(JSON.stringify(payload), {
    status: 200,
    headers: { 'content-type': 'application/json' },
  });
}

const samplePayload: FeaturedResponse = {
  language: 'en',
  series: [
    { id: 1, title: 'Alpha', firstAirDate: '2020-01-01', poster: '/img/a.jpg' },
    { id: 2, title: 'Bravo', firstAirDate: '2021-02-02', poster: '/img/b.jpg' },
    { id: 3, title: 'Charlie', firstAirDate: '2022-03-03', poster: '/img/c.jpg' },
  ],
};

describe('<FeaturedSeries /> poster attributes', () => {
  it('renders width-descriptor srcset and matching sizes for each card', async () => {
    fetchMock.mockResolvedValue(jsonResponse(samplePayload));

    const { container } = renderWithClient(
      <FeaturedSeries language="en" onSelect={vi.fn()} />,
    );

    await waitFor(() => screen.getByText('Alpha'));

    const imgs = Array.from(container.querySelectorAll('img'));
    expect(imgs).toHaveLength(3);

    for (const img of imgs) {
      const srcset = img.getAttribute('srcset') ?? '';
      expect(srcset).toContain('?size=w342 342w');
      expect(srcset).toContain('?size=w500 500w');
      expect(srcset).toContain('?size=w780 780w');
      expect(img.getAttribute('sizes')).toBe(
        '(min-width: 1024px) 210px, (min-width: 640px) 190px, 160px',
      );
    }
  });

  it('marks the first two cards as high-priority eager loads', async () => {
    fetchMock.mockResolvedValue(jsonResponse(samplePayload));

    const { container } = renderWithClient(
      <FeaturedSeries language="en" onSelect={vi.fn()} />,
    );

    await waitFor(() => screen.getByText('Alpha'));

    const [first, second, third] = Array.from(
      container.querySelectorAll('img'),
    );
    expect(first).toBeDefined();
    expect(second).toBeDefined();
    expect(third).toBeDefined();

    expect(first?.getAttribute('fetchpriority')).toBe('high');
    expect(first?.getAttribute('loading')).toBe('eager');
    expect(second?.getAttribute('fetchpriority')).toBe('high');
    expect(second?.getAttribute('loading')).toBe('eager');
    expect(third?.getAttribute('fetchpriority')).toBe('auto');
    expect(third?.getAttribute('loading')).toBe('lazy');
  });
});
