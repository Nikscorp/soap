import { useState } from 'react';
import { ArrowLeft } from 'lucide-react';
import { Header } from './components/Header';
import { SeriesCombobox } from './components/SeriesCombobox';
import { EpisodesList } from './components/EpisodesList';
import { SearchResultsPage } from './components/SearchResultsPage';
import { useUrlQuery } from './hooks/useUrlQuery';
import type { SearchResult } from './lib/types';

interface Selection {
  series: SearchResult;
  language: string;
}

export default function App() {
  const { q, setQ } = useUrlQuery();
  const [selection, setSelection] = useState<Selection | null>(null);

  const showResults = q.length > 0 && !selection;
  const showBackToResults = selection !== null && q.length > 0;

  return (
    <div className="flex min-h-dvh flex-col items-center">
      <main className="flex w-full max-w-5xl flex-col items-center px-4 sm:px-6">
        <Header />
        <section className="w-[95%] max-w-3xl rounded-md bg-white px-4 py-5 shadow-card sm:w-[80%] sm:px-7 sm:py-7">
          <p className="mb-3 text-sm font-medium text-slate-500">
            What series are you looking for?
          </p>
          <SeriesCombobox
            onSelect={(series, language) => setSelection({ series, language })}
            onSubmit={(query) => {
              setSelection(null);
              setQ(query);
            }}
          />
        </section>
        {showResults && (
          <SearchResultsPage
            q={q}
            onSelect={(series, language) => setSelection({ series, language })}
          />
        )}
        {selection && (
          <>
            {showBackToResults && (
              <div className="mt-5 w-[95%] max-w-3xl sm:w-[80%]">
                <button
                  type="button"
                  onClick={() => setSelection(null)}
                  className="inline-flex items-center gap-1.5 text-sm font-medium text-white/90 hover:text-white focus:outline-none focus-visible:ring-2 focus-visible:ring-white/70 rounded"
                >
                  <ArrowLeft className="h-4 w-4" aria-hidden="true" />
                  Back to results
                </button>
              </div>
            )}
            <EpisodesList
              key={`${selection.series.id}-${selection.language}`}
              series={selection.series}
              language={selection.language}
            />
          </>
        )}
      </main>
    </div>
  );
}
