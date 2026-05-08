import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { getEpisodesById, normalizePosterUrl, posterSrcSet } from './api';
import type { EpisodesResponse } from './types';

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

  it('appends ?size=… for proxied paths when a size is requested', () => {
    expect(normalizePosterUrl('/img/abc.jpg', 'w342')).toBe('/img/abc.jpg?size=w342');
    expect(normalizePosterUrl('abc.jpg', 'w500')).toBe('/img/abc.jpg?size=w500');
  });

  it('leaves absolute urls untouched even when a size is provided', () => {
    expect(normalizePosterUrl('https://image.tmdb.org/t/p/w92/abc.jpg', 'w500')).toBe(
      'https://image.tmdb.org/t/p/w92/abc.jpg',
    );
  });
});

describe('posterSrcSet', () => {
  it('builds a width-descriptor srcset for each requested size', () => {
    expect(posterSrcSet('/img/abc.jpg', ['w185', 'w342', 'w500', 'w780'])).toBe(
      [
        '/img/abc.jpg?size=w185 185w',
        '/img/abc.jpg?size=w342 342w',
        '/img/abc.jpg?size=w500 500w',
        '/img/abc.jpg?size=w780 780w',
      ].join(', '),
    );
  });

  it('routes bare poster paths through /img', () => {
    expect(posterSrcSet('abc.jpg', ['w185', 'w342'])).toBe(
      '/img/abc.jpg?size=w185 185w, /img/abc.jpg?size=w342 342w',
    );
  });

  it('returns empty string for empty / nullish poster', () => {
    expect(posterSrcSet(null, ['w185', 'w342'])).toBe('');
    expect(posterSrcSet(undefined, ['w185', 'w342'])).toBe('');
    expect(posterSrcSet('', ['w185', 'w342'])).toBe('');
  });

  it('returns empty string when no sizes are requested', () => {
    expect(posterSrcSet('/img/abc.jpg', [])).toBe('');
  });

  it('returns empty string for absolute URLs (descriptors meaningless)', () => {
    expect(
      posterSrcSet('https://image.tmdb.org/t/p/w92/abc.jpg', ['w185', 'w342']),
    ).toBe('');
  });
});

describe('getEpisodesById URL composition', () => {
  const fetchMock = vi.fn();
  const okResponse: EpisodesResponse = {
    episodes: [],
    title: '',
    poster: '',
    firstAirDate: '',
    defaultBest: 0,
    totalEpisodes: 0,
    availableSeasons: [],
  };

  beforeEach(() => {
    fetchMock.mockReset();
    fetchMock.mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify(okResponse), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        }),
      ),
    );
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function calledUrl(): string {
    const arg = fetchMock.mock.calls[0]?.[0];
    return typeof arg === 'string' ? arg : (arg as URL).toString();
  }

  it('omits limit and seasons when neither is provided', async () => {
    await getEpisodesById(42, 'en');
    expect(calledUrl()).toBe('/id/42?language=en');
  });

  it('appends limit when provided', async () => {
    await getEpisodesById(42, 'en', 5);
    expect(calledUrl()).toBe('/id/42?language=en&limit=5');
  });

  it('omits seasons when null, undefined, or empty', async () => {
    await getEpisodesById(42, 'en', undefined, null);
    expect(calledUrl()).toBe('/id/42?language=en');
    fetchMock.mockClear();
    await getEpisodesById(42, 'en', undefined, undefined);
    expect(calledUrl()).toBe('/id/42?language=en');
    fetchMock.mockClear();
    await getEpisodesById(42, 'en', undefined, []);
    expect(calledUrl()).toBe('/id/42?language=en');
  });

  it('appends seasons as ascending CSV', async () => {
    await getEpisodesById(42, 'en', undefined, [1]);
    expect(calledUrl()).toBe('/id/42?language=en&seasons=1');
    fetchMock.mockClear();
    await getEpisodesById(42, 'en', undefined, [3, 1, 2]);
    expect(calledUrl()).toBe('/id/42?language=en&seasons=1,2,3');
  });

  it('dedupes and drops invalid season values before serializing', async () => {
    await getEpisodesById(42, 'en', undefined, [2, 2, 1, 0, -3, Number.NaN, 3]);
    expect(calledUrl()).toBe('/id/42?language=en&seasons=1,2,3');
  });

  it('places seasons after limit in the query string', async () => {
    await getEpisodesById(42, 'en', 7, [1, 3]);
    expect(calledUrl()).toBe('/id/42?language=en&limit=7&seasons=1,3');
  });
});
