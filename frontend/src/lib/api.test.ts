import { describe, expect, it } from 'vitest';
import { normalizePosterUrl } from './api';

describe('normalizePosterUrl', () => {
  it('passes through absolute http(s) urls untouched', () => {
    expect(normalizePosterUrl('https://image.tmdb.org/t/p/w92/abc.jpg')).toBe(
      'https://image.tmdb.org/t/p/w92/abc.jpg',
    );
    expect(normalizePosterUrl('http://example.com/poster.jpg')).toBe(
      'http://example.com/poster.jpg',
    );
  });

  it('passes through paths that already point at the backend /img handler', () => {
    expect(normalizePosterUrl('/img/abc.jpg')).toBe('/img/abc.jpg');
  });

  it('routes bare poster paths through /img', () => {
    expect(normalizePosterUrl('abc.jpg')).toBe('/img/abc.jpg');
  });

  it('keeps other absolute paths same-origin', () => {
    expect(normalizePosterUrl('/something/abc.jpg')).toBe('/something/abc.jpg');
  });

  it('returns null for empty / nullish values', () => {
    expect(normalizePosterUrl(null)).toBeNull();
    expect(normalizePosterUrl(undefined)).toBeNull();
    expect(normalizePosterUrl('')).toBeNull();
  });
});
