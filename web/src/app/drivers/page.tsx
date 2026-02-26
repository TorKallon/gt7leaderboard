import { db } from '@/lib/db';
import { sql } from 'drizzle-orm';
import Link from 'next/link';
import { DriverBadge } from '@/components/driver-badge';

interface DriverListItem {
  id: string;
  display_name: string;
  is_guest: boolean;
  lap_count: number;
}

async function fetchDrivers(): Promise<DriverListItem[]> {
  try {
    const result = await db.execute(sql`
      SELECT
        d.id,
        d.display_name,
        d.is_guest,
        COALESCE(lc.lap_count, 0) AS lap_count
      FROM drivers d
      LEFT JOIN LATERAL (
        SELECT COUNT(*)::int AS lap_count
        FROM lap_records lr
        WHERE lr.driver_id = d.id
      ) lc ON true
      ORDER BY d.is_guest ASC, d.display_name ASC
    `);
    return result.rows as unknown as DriverListItem[];
  } catch {
    return [];
  }
}

export default async function DriversPage() {
  const drivers = await fetchDrivers();

  const familyDrivers = drivers.filter((d) => !d.is_guest);
  const guestDrivers = drivers.filter((d) => d.is_guest);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">Drivers</h1>

      {drivers.length === 0 ? (
        <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-8 text-center">
          <p className="text-neutral-500 mb-2">No drivers registered yet.</p>
          <p className="text-xs text-neutral-600">
            Drivers will appear here when detected by the collector or created manually.
          </p>
        </div>
      ) : (
        <>
          {/* Family members */}
          {familyDrivers.length > 0 && (
            <div>
              <h2 className="text-sm font-semibold text-neutral-400 uppercase tracking-wider mb-3">
                Family
              </h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                {familyDrivers.map((driver) => (
                  <DriverCard key={driver.id} driver={driver} />
                ))}
              </div>
            </div>
          )}

          {/* Guests */}
          {guestDrivers.length > 0 && (
            <div>
              <h2 className="text-sm font-semibold text-neutral-400 uppercase tracking-wider mb-3">
                Guests
              </h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                {guestDrivers.map((driver) => (
                  <DriverCard key={driver.id} driver={driver} />
                ))}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

function DriverCard({ driver }: { driver: DriverListItem }) {
  return (
    <Link
      href={`/drivers/${driver.id}`}
      className="block rounded-lg bg-[#1f1f1f] border border-neutral-800 p-5 hover:border-neutral-600 hover:bg-[#252525] transition-all"
    >
      <div className="flex items-center justify-between">
        <div>
          <DriverBadge name={driver.display_name} />
          {driver.is_guest && (
            <span className="ml-2 text-xs text-neutral-600">Guest</span>
          )}
        </div>
      </div>
      <div className="mt-3 text-sm text-neutral-400">
        <span className="text-neutral-300 font-medium">{driver.lap_count}</span>{' '}
        lap{driver.lap_count !== 1 ? 's' : ''}
      </div>
    </Link>
  );
}
