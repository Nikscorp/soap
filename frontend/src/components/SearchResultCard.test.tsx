import { describe, expect, it, vi } from 'vitest';
import { render } from '@testing-library/react';
import { SearchResultCard } from './SearchResultCard';
import type { SearchResult } from '@/lib/types';

const sampleResult: SearchResult = {
  id: 42009,
  title: 'Black Mirror',
  firstAirDate: '2011-12-04',
  poster: '/img/blackmirror.jpg',
  rating: 8.3,
  description: 'A British anthology series.',
};

describe('<SearchResultCard /> poster attributes', () => {
  it('renders width-descriptor srcset and matching sizes', () => {
    const { container } = render(
      <SearchResultCard result={sampleResult} onSelect={vi.fn()} />,
    );

    const img = container.querySelector('img');
    expect(img).not.toBeNull();

    const srcset = img?.getAttribute('srcset') ?? '';
    expect(srcset).toContain('?size=w185 185w');
    expect(srcset).toContain('?size=w342 342w');
    expect(srcset).not.toContain('500w');
    expect(srcset).not.toContain('780w');

    expect(img?.getAttribute('sizes')).toBe('(min-width: 640px) 90px, 80px');
    expect(img?.getAttribute('src')).toContain('?size=w185');
    expect(img?.getAttribute('loading')).toBe('lazy');
  });

  it('renders the placeholder icon when poster is missing', () => {
    const { container } = render(
      <SearchResultCard
        result={{ ...sampleResult, poster: '' }}
        onSelect={vi.fn()}
      />,
    );
    expect(container.querySelector('img')).toBeNull();
  });
});
