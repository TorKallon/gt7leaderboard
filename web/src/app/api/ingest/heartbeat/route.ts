import { NextResponse } from 'next/server';
import { validateIngestAuth } from '@/lib/ingest-auth';
import { db } from '@/lib/db';
import { collectorHeartbeats } from '@/lib/db/schema';
import { sql } from 'drizzle-orm';

export async function POST(request: Request) {
  if (!validateIngestAuth(request)) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
  }

  try {
    const body = await request.json();
    const { status, current_session_id, uptime_seconds } = body;

    // Insert new heartbeat and clean up old ones to prevent unbounded growth.
    await db.insert(collectorHeartbeats).values({
      status,
      currentSessionId: current_session_id || null,
      uptimeSeconds: uptime_seconds || null,
    });

    // Keep only the 10 most recent heartbeats.
    await db.execute(sql`
      DELETE FROM collector_heartbeats
      WHERE id NOT IN (
        SELECT id FROM collector_heartbeats
        ORDER BY received_at DESC
        LIMIT 10
      )
    `);

    return NextResponse.json({ ok: true });
  } catch (error) {
    console.error('Failed to record heartbeat:', error);
    return NextResponse.json(
      { error: 'Failed to record heartbeat' },
      { status: 500 }
    );
  }
}
