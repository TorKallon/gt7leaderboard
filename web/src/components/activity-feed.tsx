import { formatLapTime } from './lap-time';
import { DriverBadge } from './driver-badge';

export interface ActivityItem {
  id: string;
  driver_name: string | null;
  track_name: string | null;
  car_name: string | null;
  lap_time_ms: number;
  recorded_at: string;
}

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffMs = now - then;

  const seconds = Math.floor(diffMs / 1000);
  if (seconds < 60) return 'just now';

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;

  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;

  return new Date(dateStr).toLocaleDateString();
}

export function ActivityFeed({ items }: { items: ActivityItem[] }) {
  if (items.length === 0) {
    return (
      <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-6 text-center text-neutral-500">
        No recent laps recorded yet.
      </div>
    );
  }

  return (
    <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 overflow-hidden">
      <div className="px-4 py-3 border-b border-neutral-800">
        <h3 className="text-sm font-semibold text-white">Recent Activity</h3>
      </div>
      <div className="divide-y divide-neutral-800">
        {items.map((item) => (
          <div
            key={item.id}
            className="px-4 py-3 flex items-center gap-3 hover:bg-neutral-800/30 transition-colors"
          >
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 flex-wrap">
                {item.driver_name ? (
                  <DriverBadge name={item.driver_name} />
                ) : (
                  <span className="text-xs text-neutral-500 italic">
                    Unknown
                  </span>
                )}
                <span className="text-sm text-neutral-300 truncate">
                  {item.track_name ?? 'Unknown Track'}
                </span>
              </div>
              <p className="text-xs text-neutral-500 mt-0.5 truncate">
                {item.car_name ?? 'Unknown Car'}
              </p>
            </div>
            <div className="text-right shrink-0">
              <p className="font-mono tabular-nums text-sm text-white">
                {formatLapTime(item.lap_time_ms)}
              </p>
              <p className="text-xs text-neutral-500">
                {timeAgo(item.recorded_at)}
              </p>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
