import Link from 'next/link';
import { db } from '@/lib/db';
import { getRecentLaps } from '@/lib/db/queries';
import { sql } from 'drizzle-orm';
import { StatCard } from '@/components/stat-card';
import { ActivityFeed, type ActivityItem } from '@/components/activity-feed';
import { CollectorStatus, type CollectorStatusData } from '@/components/collector-status';

async function getDashboardStats() {
  try {
    const result = await db.execute(sql`
      SELECT
        (SELECT COUNT(*)::int FROM lap_records) AS total_laps,
        (SELECT COUNT(DISTINCT track_id)::int FROM lap_records) AS tracks_driven,
        (SELECT COUNT(DISTINCT driver_id)::int FROM lap_records WHERE driver_id IS NOT NULL) AS active_drivers,
        (SELECT COUNT(*)::int FROM sessions WHERE driver_id IS NULL) AS unclaimed_sessions
    `);
    return result.rows[0] as {
      total_laps: number;
      tracks_driven: number;
      active_drivers: number;
      unclaimed_sessions: number;
    };
  } catch {
    return null;
  }
}

async function getCollectorStatus(): Promise<CollectorStatusData> {
  try {
    const result = await db.execute(sql`
      SELECT status, current_session_id, uptime_seconds, received_at
      FROM collector_heartbeats
      ORDER BY received_at DESC
      LIMIT 1
    `);

    if (result.rows.length === 0) {
      return { collector_online: false, last_heartbeat: null };
    }

    const latest = result.rows[0] as {
      status: string;
      received_at: string;
      uptime_seconds: number | null;
    };
    const receivedAt = new Date(latest.received_at);
    const twoMinutesAgo = new Date(Date.now() - 2 * 60 * 1000);

    return {
      collector_online: receivedAt > twoMinutesAgo,
      last_heartbeat: receivedAt.toISOString(),
      uptime_seconds: latest.uptime_seconds,
    };
  } catch {
    return { collector_online: false, last_heartbeat: null };
  }
}

async function fetchRecentLaps(): Promise<ActivityItem[]> {
  try {
    const rows = await getRecentLaps(db, { limit: 20 });
    return rows as unknown as ActivityItem[];
  } catch {
    return [];
  }
}

export default async function DashboardPage() {
  const [stats, collectorData, recentLaps] = await Promise.all([
    getDashboardStats(),
    getCollectorStatus(),
    fetchRecentLaps(),
  ]);

  const hasData = stats !== null;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">Dashboard</h1>

      {/* Unclaimed sessions alert */}
      {hasData && stats.unclaimed_sessions > 0 && (
        <div className="rounded-lg bg-orange-900/20 border border-orange-700/40 px-4 py-3">
          <p className="text-sm text-orange-300">
            <span className="font-semibold">{stats.unclaimed_sessions}</span>{' '}
            session{stats.unclaimed_sessions !== 1 ? 's' : ''} with unknown
            driver.{' '}
            <Link href="/sessions" className="underline hover:text-orange-200">
              Review sessions
            </Link>
          </p>
        </div>
      )}

      {/* Stat cards + collector status */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          label="Total Laps"
          value={hasData ? stats.total_laps : '--'}
          subtitle={hasData ? undefined : 'No data yet'}
        />
        <StatCard
          label="Tracks Driven"
          value={hasData ? stats.tracks_driven : '--'}
          subtitle={hasData ? undefined : 'No data yet'}
        />
        <StatCard
          label="Active Drivers"
          value={hasData ? stats.active_drivers : '--'}
          subtitle={hasData ? undefined : 'No data yet'}
        />
        <CollectorStatus data={collectorData} />
      </div>

      {/* Recent activity */}
      <ActivityFeed items={recentLaps} />
    </div>
  );
}
