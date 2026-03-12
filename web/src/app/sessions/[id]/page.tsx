export const dynamic = 'force-dynamic';

import { db } from '@/lib/db';
import { sql } from 'drizzle-orm';
import { notFound } from 'next/navigation';
import { DriverBadge } from '@/components/driver-badge';
import { formatLapTime } from '@/components/lap-time';
import { ReassignDialog } from '@/components/reassign-dialog';
import { WeatherTag } from '@/components/weather-tag';
import { LapReassignDialog } from '@/components/lap-reassign-dialog';
import { DeleteSessionDialog } from '@/components/delete-session-dialog';

interface SessionDetail {
  id: string;
  driver_id: string | null;
  driver_name: string | null;
  auto_detected_driver_name: string | null;
  track_name: string | null;
  track_slug: string | null;
  car_name: string | null;
  car_manufacturer: string | null;
  started_at: string;
  ended_at: string | null;
  detection_method: string;
}

interface SessionLap {
  id: string;
  lap_number: number;
  lap_time_ms: number;
  driver_id: string | null;
  driver_name: string | null;
  weather: string;
  is_valid: boolean;
  recorded_at: string;
}

async function getSessionDetail(
  id: string
): Promise<{ session: SessionDetail; laps: SessionLap[] } | null> {
  try {
    const sessionResult = await db.execute(sql`
      SELECT
        s.id,
        s.driver_id,
        d.display_name AS driver_name,
        ad.display_name AS auto_detected_driver_name,
        t.name AS track_name,
        t.slug AS track_slug,
        s.car_id,
        c.name AS car_name,
        c.manufacturer AS car_manufacturer,
        s.started_at,
        s.ended_at,
        s.detection_method
      FROM sessions s
      LEFT JOIN drivers d ON d.id = s.driver_id
      LEFT JOIN tracks t ON t.id = s.track_id
      LEFT JOIN cars c ON c.id = s.car_id
      LEFT JOIN LATERAL (
        SELECT DISTINCT lr.auto_detected_driver_id
        FROM lap_records lr
        WHERE lr.session_id = s.id AND lr.auto_detected_driver_id IS NOT NULL
        LIMIT 1
      ) orig ON true
      LEFT JOIN drivers ad ON ad.id = orig.auto_detected_driver_id
      WHERE s.id = ${id}
    `);

    if (sessionResult.rows.length === 0) return null;

    const lapsResult = await db.execute(sql`
      SELECT
        lr.id,
        lr.lap_number,
        lr.lap_time_ms,
        lr.driver_id,
        d.display_name AS driver_name,
        lr.weather,
        lr.is_valid,
        lr.recorded_at
      FROM lap_records lr
      LEFT JOIN drivers d ON d.id = lr.driver_id
      WHERE lr.session_id = ${id}
      ORDER BY lr.lap_number ASC
    `);

    return {
      session: sessionResult.rows[0] as unknown as SessionDetail,
      laps: lapsResult.rows as unknown as SessionLap[],
    };
  } catch {
    return null;
  }
}

const DETECTION_LABELS: Record<string, string> = {
  telemetry: 'Telemetry',
  forward: 'Forward Direction',
  reverse: 'Reverse Direction',
  psn: 'PSN Detection',
  manual: 'Manual',
  unmatched: 'Unmatched',
};

export default async function SessionDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  const data = await getSessionDetail(id);
  if (!data) {
    notFound();
  }

  const { session, laps } = data;

  // Find best lap
  const validLaps = laps.filter((l) => l.is_valid);
  const bestLap = validLaps.length > 0
    ? validLaps.reduce((best, lap) =>
        lap.lap_time_ms < best.lap_time_ms ? lap : best
      )
    : null;

  return (
    <div className="space-y-6">
      {/* Session header */}
      <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-white">Session Detail</h1>
          <div className="flex flex-wrap items-center gap-3 mt-2">
            {session.driver_name ? (
              <DriverBadge name={session.driver_name} />
            ) : (
              <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-orange-900/40 text-orange-300 border border-orange-700/40">
                Unknown Driver
              </span>
            )}
            {session.auto_detected_driver_name &&
              session.auto_detected_driver_name !== session.driver_name && (
              <span className="text-xs text-neutral-500">
                (originally detected: {session.auto_detected_driver_name})
              </span>
            )}
            {session.track_name && (
              <a
                href={`/tracks/${session.track_slug}`}
                className="text-sm text-neutral-300 hover:text-white"
              >
                {session.track_name}
              </a>
            )}
            {session.car_name && (
              <span className="text-sm text-neutral-500">
                {session.car_manufacturer} {session.car_name}
              </span>
            )}
          </div>
        </div>

        <div className="flex gap-2">
          <ReassignDialog
            sessionId={session.id}
            currentDriverId={session.driver_id}
          />
          <DeleteSessionDialog sessionId={session.id} />
        </div>
      </div>

      {/* Session metadata */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-4">
          <p className="text-xs text-neutral-500 uppercase tracking-wider">
            Started
          </p>
          <p className="text-sm text-white mt-1">
            {new Date(session.started_at).toLocaleString()}
          </p>
        </div>
        <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-4">
          <p className="text-xs text-neutral-500 uppercase tracking-wider">
            Ended
          </p>
          <p className="text-sm text-white mt-1">
            {session.ended_at
              ? new Date(session.ended_at).toLocaleString()
              : 'In progress'}
          </p>
        </div>
        <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-4">
          <p className="text-xs text-neutral-500 uppercase tracking-wider">
            Detection
          </p>
          <p className="text-sm text-white mt-1">
            {DETECTION_LABELS[session.detection_method] ?? session.detection_method}
          </p>
        </div>
        <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-4">
          <p className="text-xs text-neutral-500 uppercase tracking-wider">
            Best Lap
          </p>
          <p className="text-sm text-white mt-1 font-mono tabular-nums">
            {bestLap ? formatLapTime(bestLap.lap_time_ms) : '--'}
          </p>
        </div>
      </div>

      {/* Laps table */}
      <div>
        <h2 className="text-lg font-semibold text-white mb-3">
          Laps ({laps.length})
        </h2>

        {laps.length === 0 ? (
          <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-6 text-center text-neutral-500">
            No laps recorded in this session.
          </div>
        ) : (
          <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-neutral-800 text-neutral-500 text-xs uppercase tracking-wider">
                    <th className="px-4 py-3 text-left w-16">Lap</th>
                    <th className="px-4 py-3 text-left">Driver</th>
                    <th className="px-4 py-3 text-right">Time</th>
                    <th className="px-4 py-3 text-right hidden sm:table-cell">
                      Delta
                    </th>
                    <th className="px-4 py-3 text-center hidden md:table-cell">
                      Weather
                    </th>
                    <th className="px-4 py-3 text-center hidden md:table-cell">
                      Valid
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-neutral-800/50">
                  {laps.map((lap) => {
                    const deltaMs = bestLap
                      ? lap.lap_time_ms - bestLap.lap_time_ms
                      : 0;

                    return (
                      <tr
                        key={lap.id}
                        className={`hover:bg-neutral-800/30 transition-colors ${
                          !lap.is_valid ? 'opacity-50' : ''
                        } ${
                          bestLap && lap.id === bestLap.id
                            ? 'bg-yellow-900/10'
                            : ''
                        }`}
                      >
                        <td className="px-4 py-3 font-mono text-neutral-400">
                          {lap.lap_number}
                        </td>
                        <td className="px-4 py-3">
                          <LapReassignDialog
                            lapId={lap.id}
                            currentDriverName={lap.driver_name}
                            currentDriverId={lap.driver_id}
                          />
                        </td>
                        <td className="px-4 py-3 text-right">
                          <span
                            className={`font-mono tabular-nums ${
                              bestLap && lap.id === bestLap.id
                                ? 'text-yellow-500 font-bold'
                                : 'text-white'
                            }`}
                          >
                            {formatLapTime(lap.lap_time_ms)}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right hidden sm:table-cell">
                          {deltaMs === 0 ? (
                            <span className="font-mono text-neutral-600">
                              --
                            </span>
                          ) : (
                            <span className="font-mono tabular-nums text-red-400">
                              +{formatLapTime(deltaMs)}
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-center hidden md:table-cell">
                          <WeatherTag
                            lapId={lap.id}
                            initialWeather={lap.weather}
                          />
                        </td>
                        <td className="px-4 py-3 text-center hidden md:table-cell">
                          {lap.is_valid ? (
                            <span className="text-green-500 text-xs">OK</span>
                          ) : (
                            <span className="text-red-500 text-xs">
                              Invalid
                            </span>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
