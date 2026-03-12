import { db } from '@/lib/db';
import { sql } from 'drizzle-orm';
import { AddDriverForm } from './add-driver-form';
import { EditDriverDialog } from './edit-driver-dialog';

interface CollectorInfo {
  collector_online: boolean;
  last_heartbeat: string | null;
  uptime_seconds: number | null;
  status: string | null;
}

interface DriverItem {
  id: string;
  display_name: string;
  psn_online_id: string | null;
  is_guest: boolean;
}

async function getCollectorInfo(): Promise<CollectorInfo> {
  try {
    const result = await db.execute(sql`
      SELECT status, uptime_seconds, received_at
      FROM collector_heartbeats
      ORDER BY received_at DESC
      LIMIT 1
    `);

    if (result.rows.length === 0) {
      return {
        collector_online: false,
        last_heartbeat: null,
        uptime_seconds: null,
        status: null,
      };
    }

    const row = result.rows[0] as {
      status: string;
      uptime_seconds: number | null;
      received_at: string;
    };
    const receivedAt = new Date(row.received_at);
    const twoMinutesAgo = new Date(Date.now() - 2 * 60 * 1000);

    return {
      collector_online: receivedAt > twoMinutesAgo,
      last_heartbeat: receivedAt.toISOString(),
      uptime_seconds: row.uptime_seconds,
      status: row.status,
    };
  } catch {
    return {
      collector_online: false,
      last_heartbeat: null,
      uptime_seconds: null,
      status: null,
    };
  }
}

async function getDrivers(): Promise<DriverItem[]> {
  try {
    const result = await db.execute(sql`
      SELECT id, display_name, psn_online_id, is_guest
      FROM drivers
      ORDER BY is_guest ASC, display_name ASC
    `);
    return result.rows as unknown as DriverItem[];
  } catch {
    return [];
  }
}

function formatUptime(seconds: number): string {
  const hours = Math.floor(seconds / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  if (hours > 0) return `${hours}h ${mins}m`;
  return `${mins}m`;
}

export default async function SettingsPage() {
  const [collector, drivers] = await Promise.all([
    getCollectorInfo(),
    getDrivers(),
  ]);

  return (
    <div className="space-y-8 max-w-3xl">
      <h1 className="text-2xl font-bold text-white">Settings</h1>

      {/* Collector Status */}
      <section>
        <h2 className="text-lg font-semibold text-white mb-3">
          Collector Status
        </h2>
        <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-5 space-y-3">
          <div className="flex items-center gap-2">
            <span
              className={`inline-block w-3 h-3 rounded-full ${
                collector.collector_online
                  ? 'bg-green-500 shadow-[0_0_6px_rgba(34,197,94,0.5)]'
                  : 'bg-neutral-600'
              }`}
            />
            <span className="text-sm font-medium text-white">
              {collector.collector_online ? 'Online' : 'Offline'}
            </span>
            {collector.status && (
              <span className="text-xs text-neutral-500">
                ({collector.status})
              </span>
            )}
          </div>

          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p className="text-neutral-500">Last Heartbeat</p>
              <p className="text-neutral-300">
                {collector.last_heartbeat
                  ? new Date(collector.last_heartbeat).toLocaleString()
                  : 'Never'}
              </p>
            </div>
            <div>
              <p className="text-neutral-500">Uptime</p>
              <p className="text-neutral-300">
                {collector.uptime_seconds != null
                  ? formatUptime(collector.uptime_seconds)
                  : '--'}
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Driver Management */}
      <section>
        <h2 className="text-lg font-semibold text-white mb-3">
          Driver Management
        </h2>

        {/* Add guest driver form */}
        <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-5 mb-4">
          <h3 className="text-sm font-semibold text-neutral-400 mb-3">
            Add Guest Driver
          </h3>
          <AddDriverForm />
        </div>

        {/* Driver list */}
        <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 overflow-hidden">
          <div className="px-4 py-3 border-b border-neutral-800">
            <h3 className="text-sm font-semibold text-white">
              All Drivers ({drivers.length})
            </h3>
          </div>

          {drivers.length === 0 ? (
            <div className="p-6 text-center text-neutral-500 text-sm">
              No drivers yet.
            </div>
          ) : (
            <div className="divide-y divide-neutral-800/50">
              {drivers.map((driver) => (
                <div
                  key={driver.id}
                  className="px-4 py-3 flex items-center justify-between"
                >
                  <div>
                    <span className="text-sm text-white">
                      {driver.display_name}
                    </span>
                    {driver.is_guest && (
                      <span className="ml-2 text-xs text-neutral-600">
                        Guest
                      </span>
                    )}
                    {driver.psn_online_id && (
                      <span className="ml-2 text-xs text-neutral-500">
                        PSN: {driver.psn_online_id}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-3">
                    {driver.is_guest && (
                      <EditDriverDialog
                        driverId={driver.id}
                        initialName={driver.display_name}
                        initialPsnOnlineId={driver.psn_online_id}
                      />
                    )}
                    <span className="text-xs text-neutral-600 font-mono">
                      {driver.id.slice(0, 8)}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>
    </div>
  );
}
