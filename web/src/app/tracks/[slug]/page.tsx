import { db } from '@/lib/db';
import { getTrackLeaderboard, getTrackRecords } from '@/lib/db/queries';
import { sql } from 'drizzle-orm';
import { formatLapTime } from '@/components/lap-time';
import { notFound } from 'next/navigation';
import { Suspense } from 'react';
import { CategoryTabs } from '@/components/category-tabs';
import { CarFilter } from '@/components/car-filter';
import { LeaderboardTable, type LeaderboardRow } from '@/components/leaderboard-table';

interface TrackInfo {
  id: string;
  name: string;
  layout: string;
  country: string | null;
  length_meters: number | null;
  num_corners: number | null;
  has_weather: boolean;
}

interface CarOption {
  id: number;
  name: string;
  manufacturer: string;
}

async function getTrackBySlug(slug: string): Promise<TrackInfo | null> {
  try {
    const result = await db.execute(sql`
      SELECT id, name, layout, country, length_meters, num_corners, has_weather
      FROM tracks
      WHERE slug = ${slug}
      LIMIT 1
    `);
    if (result.rows.length === 0) return null;
    return result.rows[0] as unknown as TrackInfo;
  } catch {
    return null;
  }
}

async function getTrackCars(trackId: string): Promise<CarOption[]> {
  try {
    const result = await db.execute(sql`
      SELECT DISTINCT c.id, c.name, c.manufacturer
      FROM lap_records lr
      JOIN cars c ON c.id = lr.car_id
      WHERE lr.track_id = ${trackId} AND lr.is_valid = true
      ORDER BY c.manufacturer, c.name
    `);
    return result.rows as unknown as CarOption[];
  } catch {
    return [];
  }
}

async function fetchLeaderboard(
  trackId: string,
  category?: string,
  carId?: number
): Promise<LeaderboardRow[]> {
  try {
    const rows = await getTrackLeaderboard(db, trackId, { category, carId });
    return rows as unknown as LeaderboardRow[];
  } catch {
    return [];
  }
}

export default async function TrackDetailPage({
  params,
  searchParams,
}: {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ category?: string; car_id?: string }>;
}) {
  const { slug } = await params;
  const sp = await searchParams;

  const track = await getTrackBySlug(slug);
  if (!track) {
    notFound();
  }

  const category = sp.category ?? undefined;
  const carId = sp.car_id ? parseInt(sp.car_id, 10) : undefined;

  const [leaderboard, cars, records] = await Promise.all([
    fetchLeaderboard(track.id, category, carId),
    getTrackCars(track.id),
    getTrackRecords(db, track.id, ['Gr.3']),
  ]);

  return (
    <div className="space-y-6">
      {/* Track header */}
      <div>
        <h1 className="text-2xl font-bold text-white">{track.name}</h1>
        {track.layout && (
          <p className="text-neutral-400 mt-0.5">{track.layout}</p>
        )}
        <div className="flex items-center gap-4 mt-2 text-sm text-neutral-500">
          {track.country && <span>{track.country}</span>}
          {track.length_meters && (
            <span>{(track.length_meters / 1000).toFixed(2)} km</span>
          )}
          {track.num_corners && (
            <span>
              {track.num_corners} corner{track.num_corners !== 1 ? 's' : ''}
            </span>
          )}
          {track.has_weather && (
            <span className="text-blue-400">Dynamic Weather</span>
          )}
        </div>
      </div>

      {/* Track Records */}
      {records.some(r => r.lap_time_ms !== null) && (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          {records.map((rec) => (
            <div
              key={rec.label}
              className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-4"
            >
              <div className="text-xs text-neutral-500 mb-2">
                {rec.label} Record
              </div>
              {rec.lap_time_ms != null ? (
                <div className="flex items-center justify-between">
                  <div>
                    <span className="font-mono tabular-nums text-lg text-yellow-500 font-bold">
                      {formatLapTime(rec.lap_time_ms)}
                    </span>
                    <div className="text-sm text-neutral-400 mt-0.5">
                      {rec.driver_name}
                    </div>
                  </div>
                  <div className="text-right text-xs text-neutral-500">
                    {rec.car_name}
                  </div>
                </div>
              ) : (
                <span className="text-sm text-neutral-600">No records yet</span>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Filters */}
      <div className="flex flex-col sm:flex-row sm:items-center gap-3">
        <Suspense fallback={null}>
          <CategoryTabs />
        </Suspense>
        <Suspense fallback={null}>
          <CarFilter cars={cars} />
        </Suspense>
      </div>

      {/* Leaderboard */}
      <LeaderboardTable rows={leaderboard} />
    </div>
  );
}
