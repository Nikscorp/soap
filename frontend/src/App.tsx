import { useState } from 'react';
import { Header } from './components/Header';
import { SeriesCombobox } from './components/SeriesCombobox';
import { EpisodesList } from './components/EpisodesList';
import type { SearchResult } from './lib/types';

interface Selection {
  series: SearchResult;
  language: string;
}

export default function App() {
  const [selection, setSelection] = useState<Selection | null>(null);

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
          />
        </section>
        {selection && <EpisodesList series={selection.series} language={selection.language} />}
      </main>
    </div>
  );
}
