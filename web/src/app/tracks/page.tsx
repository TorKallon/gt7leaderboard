import { db } from '@/lib/db';
import { getTrackList } from '@/lib/db/queries';
import { TrackSearch } from '@/components/track-search';
import type { TrackCardData } from '@/components/track-card';

async function fetchTracks(): Promise<TrackCardData[]> {
  try {
    const rows = await getTrackList(db);
    return rows as unknown as TrackCardData[];
  } catch {
    return [];
  }
}

export default async function TracksPage() {
  const tracks = await fetchTracks();

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">Tracks</h1>

      {tracks.length === 0 ? (
        <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-8 text-center">
          <p className="text-neutral-500 mb-2">No tracks recorded yet.</p>
          <p className="text-xs text-neutral-600">
            Tracks will appear here once the collector starts sending lap data.
          </p>
        </div>
      ) : (
        <TrackSearch tracks={tracks} />
      )}
    </div>
  );
}
