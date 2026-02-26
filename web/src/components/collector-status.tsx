export interface CollectorStatusData {
  collector_online: boolean;
  last_heartbeat: string | null;
  uptime_seconds?: number | null;
  psn_token_expiry?: string | null;
}

function formatLastSeen(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diffMs = now - then;

  const minutes = Math.floor(diffMs / 1000 / 60);
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes} min ago`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;

  return new Date(dateStr).toLocaleDateString();
}

function formatUptime(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

function daysUntil(dateStr: string): number {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  return Math.floor((then - now) / (1000 * 60 * 60 * 24));
}

export function CollectorStatus({ data }: { data: CollectorStatusData }) {
  const psnDaysRemaining = data.psn_token_expiry
    ? daysUntil(data.psn_token_expiry)
    : null;

  return (
    <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-5">
      <div className="flex items-center gap-2 mb-3">
        <span
          className={`inline-block w-2.5 h-2.5 rounded-full ${
            data.collector_online
              ? 'bg-green-500 shadow-[0_0_6px_rgba(34,197,94,0.5)]'
              : 'bg-neutral-600'
          }`}
        />
        <span className="text-sm font-semibold text-white">
          Collector {data.collector_online ? 'Online' : 'Offline'}
        </span>
      </div>

      <div className="space-y-1.5 text-sm">
        {data.last_heartbeat && (
          <p className="text-neutral-400">
            Last seen:{' '}
            <span className="text-neutral-300">
              {formatLastSeen(data.last_heartbeat)}
            </span>
          </p>
        )}
        {data.uptime_seconds != null && data.uptime_seconds > 0 && (
          <p className="text-neutral-400">
            Uptime:{' '}
            <span className="text-neutral-300">
              {formatUptime(data.uptime_seconds)}
            </span>
          </p>
        )}
      </div>

      {psnDaysRemaining !== null && psnDaysRemaining <= 7 && (
        <div className="mt-3 rounded-md bg-orange-900/30 border border-orange-700/40 px-3 py-2">
          <p className="text-xs text-orange-300">
            PSN token expires in {psnDaysRemaining} day
            {psnDaysRemaining !== 1 ? 's' : ''}
          </p>
        </div>
      )}
    </div>
  );
}
