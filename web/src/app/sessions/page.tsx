import { db } from '@/lib/db';
import { sql } from 'drizzle-orm';
import Link from 'next/link';
import { DriverBadge } from '@/components/driver-badge';
import { SessionLoadMore } from './load-more';

interface SessionListItem {
  id: string;
  driver_id: string | null;
  driver_name: string | null;
  track_name: string | null;
  car_name: string | null;
  car_manufacturer: string | null;
  started_at: string;
  ended_at: string | null;
  detection_method: string;
  lap_count: number;
}

async function fetchSessions(limit: number): Promise<SessionListItem[]> {
  try {
    const result = await db.execute(sql`
      SELECT
        s.id,
        s.driver_id,
        d.display_name AS driver_name,
        t.name AS track_name,
        c.name AS car_name,
        c.manufacturer AS car_manufacturer,
        s.started_at,
        s.ended_at,
        s.detection_method,
        COALESCE(lc.lap_count, 0) AS lap_count
      FROM sessions s
      LEFT JOIN drivers d ON d.id = s.driver_id
      LEFT JOIN tracks t ON t.id = s.track_id
      LEFT JOIN cars c ON c.id = s.car_id
      LEFT JOIN LATERAL (
        SELECT COUNT(*)::int AS lap_count
        FROM lap_records lr
        WHERE lr.session_id = s.id
      ) lc ON true
      ORDER BY s.started_at DESC
      LIMIT ${limit}
    `);
    return result.rows as unknown as SessionListItem[];
  } catch {
    return [];
  }
}

const DETECTION_BADGES: Record<string, { label: string; className: string }> = {
  forward: { label: 'GPS Match', className: 'bg-emerald-900/40 text-emerald-300 border-emerald-700/40' },
  reverse: { label: 'GPS (Reverse)', className: 'bg-teal-900/40 text-teal-300 border-teal-700/40' },
  psn: { label: 'PSN', className: 'bg-blue-900/40 text-blue-300 border-blue-700/40' },
  schedule: { label: 'Schedule', className: 'bg-purple-900/40 text-purple-300 border-purple-700/40' },
  manual: { label: 'Manual', className: 'bg-green-900/40 text-green-300 border-green-700/40' },
  unmatched: { label: 'Unmatched', className: 'bg-neutral-800 text-neutral-400 border-neutral-700' },
};

export default async function SessionsPage() {
  const sessions = await fetchSessions(30);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">Sessions</h1>

      {sessions.length === 0 ? (
        <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-8 text-center">
          <p className="text-neutral-500 mb-2">No sessions recorded yet.</p>
          <p className="text-xs text-neutral-600">
            Sessions will appear here when the collector detects GT7 activity.
          </p>
        </div>
      ) : (
        <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-neutral-800 text-neutral-500 text-xs uppercase tracking-wider">
                  <th className="px-4 py-3 text-left">Driver</th>
                  <th className="px-4 py-3 text-left">Track</th>
                  <th className="px-4 py-3 text-left hidden sm:table-cell">Car</th>
                  <th className="px-4 py-3 text-center hidden md:table-cell">Laps</th>
                  <th className="px-4 py-3 text-center hidden md:table-cell">Detection</th>
                  <th className="px-4 py-3 text-right">Date</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-neutral-800/50">
                {sessions.map((session) => {
                  const badge = DETECTION_BADGES[session.detection_method] ?? DETECTION_BADGES.unmatched;
                  const isUnknown = session.driver_id === null;

                  return (
                    <tr
                      key={session.id}
                      className={`hover:bg-neutral-800/30 transition-colors ${
                        isUnknown ? 'bg-orange-900/5' : ''
                      }`}
                    >
                      <td className="px-4 py-3">
                        <Link href={`/sessions/${session.id}`}>
                          {session.driver_name ? (
                            <DriverBadge name={session.driver_name} />
                          ) : (
                            <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-orange-900/40 text-orange-300 border border-orange-700/40">
                              Unknown Driver
                            </span>
                          )}
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-neutral-300">
                        <Link
                          href={`/sessions/${session.id}`}
                          className="hover:text-white"
                        >
                          {session.track_name ?? 'Unknown Track'}
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-neutral-400 hidden sm:table-cell truncate max-w-[200px]">
                        {session.car_name ?? 'Unknown'}
                      </td>
                      <td className="px-4 py-3 text-center text-neutral-300 hidden md:table-cell">
                        {session.lap_count}
                      </td>
                      <td className="px-4 py-3 text-center hidden md:table-cell">
                        <span
                          className={`inline-flex items-center px-2 py-0.5 rounded text-xs border ${badge.className}`}
                        >
                          {badge.label}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-right text-neutral-500 whitespace-nowrap">
                        {new Date(session.started_at).toLocaleDateString()}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          {sessions.length >= 30 && (
            <div className="border-t border-neutral-800 p-4 text-center">
              <SessionLoadMore initialCount={30} />
            </div>
          )}
        </div>
      )}
    </div>
  );
}
