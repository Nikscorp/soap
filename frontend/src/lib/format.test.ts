import { describe, expect, it } from 'vitest';
import { formatEpisodeCode, formatRating, yearFromAirDate } from './format';

describe('format helpers', () => {
  it('extracts year from a yyyy-mm-dd air date', () => {
    expect(yearFromAirDate('2011-12-04')).toBe('2011');
    expect(yearFromAirDate('')).toBe('');
    expect(yearFromAirDate(undefined)).toBe('');
    expect(yearFromAirDate('weird')).toBe('weird');
  });

  it('formats ratings with one decimal', () => {
    expect(formatRating(7.504)).toBe('7.5');
    expect(formatRating(7.85)).toBe('7.9');
    expect(formatRating(0)).toBe('0.0');
    expect(formatRating(null)).toBe('—');
    expect(formatRating(undefined)).toBe('—');
  });

  it('formats episode codes', () => {
    expect(formatEpisodeCode(1, 4)).toBe('S1E4');
    expect(formatEpisodeCode(10, 12)).toBe('S10E12');
  });
});
