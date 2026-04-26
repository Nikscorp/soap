import { useQuery } from '@tanstack/react-query';
import { searchSeries } from '@/lib/api';
import type { SearchResponse, SearchResult } from '@/lib/types';
import { SearchResultCard } from './SearchResultCard';
import { Spinner } from './Spinner';
import { ErrorState } from './ErrorState';
import { EmptyState } from './EmptyState';

interface Props {
  q: string;
  onSelect: (result: SearchResult, language: string) => void;
}

// SearchResultsPage owns the dedicated /?q=… view: a card per match with
// poster, title, year, rating, and description. The query key matches the
// one used by SeriesCombobox so the autocomplete and the page share a cache
// entry — Enter from the combobox does not trigger a second fetch.
export function SearchResultsPage({ q, onSelect }: Props) {
  const query = useQuery<SearchResponse>({
    queryKey: ['search', q],
    queryFn: ({ signal }) => searchSeries(q, signal),
    enabled: q.length > 0,
  });

  const results = query.data?.searchResults ?? [];
  const language = query.data?.language ?? 'en';

  return (
    <section
      className="mx-auto mt-5 mb-10 w-[95%] max-w-3xl overflow-hidden rounded-md bg-white shadow-card sm:w-[80%]"
      aria-busy={query.isPending}
      aria-labelledby="search-results-heading"
    >
      <header className="border-b border-slate-100 px-5 py-4 sm:px-6">
        <h2 id="search-results-heading" className="text-sm font-medium text-slate-500">
          Results for{' '}
          <span className="font-semibold text-slate-900">&ldquo;{q}&rdquo;</span>
        </h2>
      </header>
      {query.isPending && <Spinner label="Searching…" />}
      {query.isError && <ErrorState>Service unavailable</ErrorState>}
      {query.isSuccess && results.length === 0 && (
        <EmptyState>No series match this query.</EmptyState>
      )}
      {query.isSuccess && results.length > 0 && (
        <ul className="divide-y divide-slate-100">
          {results.map((result) => (
            <SearchResultCard
              key={result.id}
              result={result}
              onSelect={(r) => onSelect(r, language)}
            />
          ))}
        </ul>
      )}
    </section>
  );
}
