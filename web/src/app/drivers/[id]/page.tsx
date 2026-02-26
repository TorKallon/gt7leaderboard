import { db } from '@/lib/db';
import { getDriverStats, getRecentLaps } from '@/lib/db/queries';
import { sql } from 'drizzle-orm';
import { notFound } from 'next/navigation';
import { StatCard } from '@/components/stat-card';
import { formatLapTime } from '@/components/lap-time';
import { ActivityFeed, type ActivityItem } from '@/components/activity-feed';

interface DriverInfo {
  id: string;
  display_name: string;
  psn_online_id: string | null;
  is_guest: boolean;
  created_at: string;
}

interface PersonalRecord {
  track_name: string;
  track_slug: string;
  lap_time_ms: number;
  car_name: string;
  achieved_at: string;
}

async function getDriver(id: string): Promise<DriverInfo | null> {
  try {
    const result = await db.execute(sql`
      SELECT id, display_name, psn_online_id, is_guest, created_at
      FROM drivers
      WHERE id = ${id}
      LIMIT 1
    `);
    if (result.rows.length === 0) return null;
    return result.rows[0] as unknown as DriverInfo;
  } catch {
    return null;
  }
}

interface DriverStatsData {
  total_laps: number;
  tracks_driven: number;
  favorite_track: string | null;
  favorite_track_slug: string | null;
  favorite_car: string | null;
  favorite_car_id: number | null;
}

async function fetchDriverStats(driverId: string): Promise<DriverStatsData> {
  try {
    const raw = await getDriverStats(db, driverId);
    return raw as unknown as DriverStatsData;
  } catch {
    return {
      total_laps: 0,
      tracks_driven: 0,
      favorite_track: null,
      favorite_track_slug: null,
      favorite_car: null,
      favorite_car_id: null,
    };
  }
}

async function getPersonalRecords(driverId: string): Promise<PersonalRecord[]> {
  try {
    const result = await db.execute(sql`
      WITH ranked AS (
        SELECT
          lr.lap_time_ms,
          lr.recorded_at,
          lr.track_id,
          t.name AS track_name,
          t.slug AS track_slug,
          c.name AS car_name,
          ROW_NUMBER() OVER (PARTITION BY lr.track_id ORDER BY lr.lap_time_ms ASC) AS rn
        FROM lap_records lr
        JOIN tracks t ON t.id = lr.track_id
        JOIN cars c ON c.id = lr.car_id
        WHERE lr.driver_id = ${driverId} AND lr.is_valid = true
      )
      SELECT track_name, track_slug, lap_time_ms, car_name, recorded_at AS achieved_at
      FROM ranked
      WHERE rn = 1
      ORDER BY track_name ASC
    `);
    return result.rows as unknown as PersonalRecord[];
  } catch {
    return [];
  }
}

async function fetchRecentLaps(driverId: string): Promise<ActivityItem[]> {
  try {
    const rows = await getRecentLaps(db, { driverId, limit: 20 });
    return rows as unknown as ActivityItem[];
  } catch {
    return [];
  }
}

export default async function DriverDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  const driver = await getDriver(id);
  if (!driver) {
    notFound();
  }

  const [stats, personalRecords, recentLaps] = await Promise.all([
    fetchDriverStats(id),
    getPersonalRecords(id),
    fetchRecentLaps(id),
  ]);

  return (
    <div className="space-y-6">
      {/* Driver header */}
      <div>
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold text-white">
            {driver.display_name}
          </h1>
          {driver.is_guest && (
            <span className="text-xs bg-neutral-800 text-neutral-400 px-2 py-0.5 rounded">
              Guest
            </span>
          )}
        </div>
        {driver.psn_online_id && (
          <p className="text-sm text-neutral-500 mt-0.5">
            PSN: {driver.psn_online_id}
          </p>
        )}
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Total Laps" value={stats.total_laps} />
        <StatCard label="Tracks Driven" value={stats.tracks_driven} />
        <StatCard
          label="Favorite Track"
          value={stats.favorite_track ?? '--'}
        />
        <StatCard
          label="Favorite Car"
          value={stats.favorite_car ?? '--'}
        />
      </div>

      {/* Personal Records */}
      <div>
        <h2 className="text-lg font-semibold text-white mb-3">
          Personal Records
        </h2>
        {personalRecords.length === 0 ? (
          <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-6 text-center text-neutral-500">
            No records yet.
          </div>
        ) : (
          <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-neutral-800 text-neutral-500 text-xs uppercase tracking-wider">
                    <th className="px-4 py-3 text-left">Track</th>
                    <th className="px-4 py-3 text-right">Best Time</th>
                    <th className="px-4 py-3 text-left hidden sm:table-cell">
                      Car
                    </th>
                    <th className="px-4 py-3 text-right hidden md:table-cell">
                      Date
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-neutral-800/50">
                  {personalRecords.map((record) => (
                    <tr
                      key={record.track_slug}
                      className="hover:bg-neutral-800/30 transition-colors"
                    >
                      <td className="px-4 py-3">
                        <a
                          href={`/tracks/${record.track_slug}`}
                          className="text-white hover:text-neutral-300"
                        >
                          {record.track_name}
                        </a>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <span className="font-mono tabular-nums text-white">
                          {formatLapTime(record.lap_time_ms)}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-neutral-400 hidden sm:table-cell">
                        {record.car_name}
                      </td>
                      <td className="px-4 py-3 text-right text-neutral-500 hidden md:table-cell">
                        {new Date(record.achieved_at).toLocaleDateString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>

      {/* Recent laps */}
      <div>
        <h2 className="text-lg font-semibold text-white mb-3">Recent Laps</h2>
        <ActivityFeed items={recentLaps} />
      </div>
    </div>
  );
}
