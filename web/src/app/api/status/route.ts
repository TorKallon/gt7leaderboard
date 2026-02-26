import { NextResponse } from 'next/server';
import { db } from '@/lib/db';
import { collectorHeartbeats } from '@/lib/db/schema';
import { desc } from 'drizzle-orm';

export async function GET() {
  try {
    const heartbeats = await db
      .select()
      .from(collectorHeartbeats)
      .orderBy(desc(collectorHeartbeats.receivedAt))
      .limit(1);

    if (heartbeats.length === 0) {
      return NextResponse.json({
        collector_online: false,
        last_heartbeat: null,
        current_session_id: null,
      });
    }

    const latest = heartbeats[0];
    const twoMinutesAgo = new Date(Date.now() - 2 * 60 * 1000);
    const isOnline = latest.receivedAt > twoMinutesAgo;

    return NextResponse.json({
      collector_online: isOnline,
      last_heartbeat: latest.receivedAt.toISOString(),
      current_session_id: latest.currentSessionId,
    });
  } catch (error) {
    console.error('Failed to get status:', error);
    return NextResponse.json(
      { error: 'Failed to get status' },
      { status: 500 }
    );
  }
}
