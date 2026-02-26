import { NextResponse } from 'next/server';
import { db } from '@/lib/db';
import { sql } from 'drizzle-orm';

export async function GET(request: Request) {
  try {
    const url = new URL(request.url);
    const driverId = url.searchParams.get('driver_id');
    const limit = parseInt(url.searchParams.get('limit') ?? '20', 10);
    const offset = parseInt(url.searchParams.get('offset') ?? '0', 10);

    const driverFilter = driverId
      ? sql`WHERE s.driver_id = ${driverId}`
      : sql``;

    const result = await db.execute(sql`
      SELECT
        s.id,
        s.driver_id,
        d.display_name AS driver_name,
        s.track_id,
        t.name AS track_name,
        t.slug AS track_slug,
        s.car_id,
        c.name AS car_name,
        c.manufacturer AS car_manufacturer,
        s.started_at,
        s.ended_at,
        s.detection_method,
        s.created_at,
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
      ${driverFilter}
      ORDER BY s.started_at DESC
      LIMIT ${limit}
      OFFSET ${offset}
    `);

    return NextResponse.json({ sessions: result.rows });
  } catch (error) {
    console.error('Failed to get sessions:', error);
    return NextResponse.json(
      { error: 'Failed to get sessions' },
      { status: 500 }
    );
  }
}
