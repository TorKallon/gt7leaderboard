'use client';

import { useState } from 'react';
import { TrackCard, type TrackCardData } from './track-card';

export function TrackSearch({ tracks }: { tracks: TrackCardData[] }) {
  const [query, setQuery] = useState('');

  const filtered = query
    ? tracks.filter(
        (t) =>
          t.name.toLowerCase().includes(query.toLowerCase()) ||
          (t.layout && t.layout.toLowerCase().includes(query.toLowerCase())) ||
          (t.country && t.country.toLowerCase().includes(query.toLowerCase()))
      )
    : tracks;

  return (
    <div>
      <input
        type="text"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search tracks..."
        className="w-full max-w-md rounded-md bg-neutral-900 border border-neutral-700 text-white text-sm px-4 py-2 mb-6 focus:outline-none focus:ring-1 focus:ring-neutral-500 placeholder:text-neutral-600"
      />

      {filtered.length === 0 ? (
        <p className="text-neutral-500 text-center py-8">
          No tracks found matching &quot;{query}&quot;
        </p>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map((track) => (
            <TrackCard key={track.slug} track={track} />
          ))}
        </div>
      )}
    </div>
  );
}
