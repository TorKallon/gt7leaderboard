import { formatLapTime } from './lap-time';
import { DriverBadge } from './driver-badge';

export interface LeaderboardRow {
  rank: number;
  driver_name: string;
  lap_time_ms: number;
  car_name: string;
  car_manufacturer?: string;
  delta_to_leader_ms: number;
  achieved_at: string;
}

export function LeaderboardTable({ rows }: { rows: LeaderboardRow[] }) {
  if (rows.length === 0) {
    return (
      <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-8 text-center text-neutral-500">
        No lap times recorded yet.
      </div>
    );
  }

  return (
    <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-neutral-800 text-neutral-500 text-xs uppercase tracking-wider">
              <th className="px-4 py-3 text-left w-12">#</th>
              <th className="px-4 py-3 text-left">Driver</th>
              <th className="px-4 py-3 text-right">Time</th>
              <th className="px-4 py-3 text-left hidden sm:table-cell">Car</th>
              <th className="px-4 py-3 text-right hidden md:table-cell">Delta</th>
              <th className="px-4 py-3 text-right hidden lg:table-cell">Date</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-neutral-800/50">
            {rows.map((row) => (
              <tr
                key={`${row.rank}-${row.driver_name}`}
                className="hover:bg-neutral-800/30 transition-colors"
              >
                <td className="px-4 py-3">
                  <span
                    className={`font-mono font-bold ${
                      row.rank === 1
                        ? 'text-yellow-500'
                        : row.rank === 2
                          ? 'text-neutral-400'
                          : row.rank === 3
                            ? 'text-orange-700'
                            : 'text-neutral-600'
                    }`}
                  >
                    {row.rank}
                  </span>
                </td>
                <td className="px-4 py-3">
                  <DriverBadge name={row.driver_name} />
                </td>
                <td className="px-4 py-3 text-right">
                  <span
                    className={`font-mono tabular-nums ${
                      row.rank === 1 ? 'text-yellow-500 font-bold' : 'text-white'
                    }`}
                  >
                    {formatLapTime(row.lap_time_ms)}
                  </span>
                </td>
                <td className="px-4 py-3 text-neutral-400 hidden sm:table-cell truncate max-w-[200px]">
                  {row.car_name}
                </td>
                <td className="px-4 py-3 text-right hidden md:table-cell">
                  {row.delta_to_leader_ms === 0 ? (
                    <span className="font-mono text-neutral-600">--</span>
                  ) : (
                    <span className="font-mono tabular-nums text-red-400">
                      +{formatLapTime(row.delta_to_leader_ms)}
                    </span>
                  )}
                </td>
                <td className="px-4 py-3 text-right text-neutral-500 hidden lg:table-cell">
                  {new Date(row.achieved_at).toLocaleDateString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
