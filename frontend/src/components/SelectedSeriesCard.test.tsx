import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/react';
import { SelectedSeriesCard } from './SelectedSeriesCard';

describe('<SelectedSeriesCard /> poster attributes', () => {
  it('renders width-descriptor srcset, sizes, and high fetch priority', () => {
    const { container } = render(
      <SelectedSeriesCard
        title="Black Mirror"
        poster="/img/blackmirror.jpg"
        firstAirDate="2011-12-04"
        description="A British anthology series."
      />,
    );

    const img = container.querySelector('img');
    expect(img).not.toBeNull();

    const srcset = img?.getAttribute('srcset') ?? '';
    expect(srcset).toContain('?size=w185 185w');
    expect(srcset).toContain('?size=w342 342w');
    expect(srcset).not.toContain('500w');
    expect(srcset).not.toContain('780w');

    expect(img?.getAttribute('sizes')).toBe('(min-width: 640px) 96px, 80px');
    expect(img?.getAttribute('src')).toContain('?size=w185');
    expect(img?.getAttribute('fetchpriority')).toBe('high');
    expect(img?.getAttribute('loading')).toBeNull();
  });

  it('renders the placeholder icon when poster is missing', () => {
    const { container } = render(
      <SelectedSeriesCard
        title="Black Mirror"
        poster=""
        firstAirDate="2011-12-04"
      />,
    );
    expect(container.querySelector('img')).toBeNull();
  });
});
