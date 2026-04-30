import { useQuery } from '@tanstack/react-query';
import { getMeta } from '../lib/api';

// Required by IMDb's "Can I use IMDb data in my software?" policy when the
// dataset dumps are the source of any user-visible field. Verbatim string
// comes straight from the policy page; do not paraphrase.
const IMDB_ATTRIBUTION =
  'Information courtesy of IMDb (https://www.imdb.com). Used with permission.';

// Attribution is a tiny footer that conditionally renders the IMDb credit
// line only when the server is configured with `LAZYSOAP_RATINGS_SOURCE=imdb`.
// In TMDB mode it renders nothing, keeping the layout identical to the
// pre-IMDb-integration UI.
export function Attribution() {
  const { data } = useQuery({
    queryKey: ['meta'],
    queryFn: ({ signal }) => getMeta(signal),
    staleTime: 5 * 60 * 1000,
  });

  if (data?.ratingsSource !== 'imdb') return null;

  return (
    <footer className="mt-6 mb-3 px-4 text-center text-xs text-white/60">
      <p>{IMDB_ATTRIBUTION}</p>
    </footer>
  );
}
