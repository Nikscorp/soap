export function yearFromAirDate(airDate: string | null | undefined): string {
  if (!airDate) return '';
  const match = /^(\d{4})/.exec(airDate);
  return match ? match[1]! : airDate;
}

export function formatRating(rating: number | null | undefined): string {
  if (rating == null || Number.isNaN(rating)) return '—';
  return (Math.round(rating * 10) / 10).toFixed(1);
}

export function formatEpisodeCode(season: number, number: number): string {
  return `S${season}E${number}`;
}
