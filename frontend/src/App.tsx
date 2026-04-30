import { useState } from 'react';
import { ArrowLeft } from 'lucide-react';
import { Header } from './components/Header';
import { SeriesCombobox } from './components/SeriesCombobox';
import { EpisodesList } from './components/EpisodesList';
import { SearchResultsPage } from './components/SearchResultsPage';
import { FeaturedSeries } from './components/FeaturedSeries';
import { Attribution } from './components/Attribution';
import { useUrlState } from './hooks/useUrlState';
import type { SearchResult } from './lib/types';

interface SeriesHint {
  id: number;
  title: string;
  poster: string;
  firstAirDate: string;
}

export default function App() {
  const { q, id, lang, best, setUrlState } = useUrlState();
  // Hint carries the user's clicked SearchResult so SelectedSeriesCard can
  // render instantly without waiting for /id/{id}. Tagged with `id` so a stale
  // hint (after back/forward or a direct URL hit) is filtered out at render.
  const [hint, setHint] = useState<SeriesHint | null>(null);
  const activeHint = hint && hint.id === id ? hint : null;
  // Bumped on home-click to remount SeriesCombobox so its internal input
  // state (intentionally preserved across URL changes) is reset to empty.
  const [comboboxResetKey, setComboboxResetKey] = useState(0);

  const showEpisodes = id !== null;
  const showResults = !showEpisodes && q.length > 0;
  const showBackToResults = showEpisodes && q.length > 0;

  const handleSelect = (series: SearchResult, language: string) => {
    setHint({
      id: series.id,
      title: series.title,
      poster: series.poster,
      firstAirDate: series.firstAirDate,
    });
    setUrlState({ id: series.id, lang: language, best: null });
  };

  const handleSubmit = (query: string) => {
    setUrlState({ q: query, id: null, lang: '', best: null });
  };

  const handleBack = () => {
    setUrlState({ id: null, lang: '', best: null });
  };

  const handleHome = () => {
    setHint(null);
    setComboboxResetKey((k) => k + 1);
    setUrlState({ q: '', id: null, lang: '', best: null });
  };

  return (
    <div className="flex min-h-dvh flex-col items-center">
      <main className="flex w-full max-w-5xl flex-col items-center px-4 sm:px-6">
        <Header onHomeClick={handleHome} />
        <section className="w-[95%] max-w-3xl rounded-md bg-white px-4 py-5 shadow-card sm:w-[80%] sm:px-7 sm:py-7">
          <p className="mb-3 text-sm font-medium text-slate-500">
            What series are you looking for?
          </p>
          <SeriesCombobox
            key={comboboxResetKey}
            onSelect={handleSelect}
            onSubmit={handleSubmit}
          />
        </section>
        {!showEpisodes && !showResults && (
          <FeaturedSeries language={lang || 'en'} onSelect={handleSelect} />
        )}
        {showResults && <SearchResultsPage q={q} onSelect={handleSelect} />}
        {showEpisodes && (
          <>
            {showBackToResults && (
              <div className="mt-5 w-[95%] max-w-3xl sm:w-[80%]">
                <button
                  type="button"
                  onClick={handleBack}
                  className="inline-flex items-center gap-1.5 text-sm font-medium text-white/90 hover:text-white focus:outline-none focus-visible:ring-2 focus-visible:ring-white/70 rounded"
                >
                  <ArrowLeft className="h-4 w-4" aria-hidden="true" />
                  Back to results
                </button>
              </div>
            )}
            <EpisodesList
              key={`${id}-${lang}`}
              seriesId={id}
              language={lang}
              best={best}
              hint={activeHint}
              onBestChange={(next) => setUrlState({ best: next }, { replace: true })}
            />
          </>
        )}
      </main>
      <Attribution />
    </div>
  );
}
