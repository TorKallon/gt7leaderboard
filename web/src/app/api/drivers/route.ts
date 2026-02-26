import { NextResponse } from 'next/server';
import { db } from '@/lib/db';
import { drivers } from '@/lib/db/schema';
import { sql } from 'drizzle-orm';

export async function GET() {
  try {
    const result = await db.execute(sql`
      SELECT
        d.id,
        d.psn_account_id,
        d.psn_online_id,
        d.display_name,
        d.is_guest,
        d.created_at,
        d.updated_at,
        COALESCE(lc.lap_count, 0) AS lap_count
      FROM drivers d
      LEFT JOIN LATERAL (
        SELECT COUNT(*)::int AS lap_count
        FROM lap_records lr
        WHERE lr.driver_id = d.id
      ) lc ON true
      ORDER BY d.display_name ASC
    `);

    return NextResponse.json({ drivers: result.rows });
  } catch (error) {
    console.error('Failed to get drivers:', error);
    return NextResponse.json(
      { error: 'Failed to get drivers' },
      { status: 500 }
    );
  }
}

export async function POST(request: Request) {
  try {
    const body = await request.json();
    const { display_name, psn_online_id } = body;

    if (!display_name) {
      return NextResponse.json(
        { error: 'display_name is required' },
        { status: 400 }
      );
    }

    const result = await db
      .insert(drivers)
      .values({
        displayName: display_name,
        psnOnlineId: psn_online_id ?? null,
      })
      .returning();

    return NextResponse.json({ driver: result[0] });
  } catch (error) {
    console.error('Failed to create driver:', error);
    return NextResponse.json(
      { error: 'Failed to create driver' },
      { status: 500 }
    );
  }
}
