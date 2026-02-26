import { NextResponse } from 'next/server';
import { validateIngestAuth } from '@/lib/ingest-auth';
import { db } from '@/lib/db';
import { sessions, tracks } from '@/lib/db/schema';
import { eq } from 'drizzle-orm';

export async function POST(request: Request) {
  if (!validateIngestAuth(request)) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
  }

  try {
    const body = await request.json();
    const { driver_id, track_slug, car_id, started_at, detection_method } = body;

    let trackId: string | null = null;

    if (track_slug) {
      const trackRows = await db
        .select({ id: tracks.id })
        .from(tracks)
        .where(eq(tracks.slug, track_slug))
        .limit(1);
      if (trackRows.length > 0) {
        trackId = trackRows[0].id;
      }
    }

    const result = await db
      .insert(sessions)
      .values({
        driverId: driver_id || null,
        trackId: trackId,
        carId: car_id || null,
        startedAt: new Date(started_at),
        detectionMethod: detection_method || 'unmatched',
      })
      .returning({ id: sessions.id });

    return NextResponse.json({ session_id: result[0].id });
  } catch (error) {
    console.error('Failed to create session:', error);
    return NextResponse.json(
      { error: 'Failed to create session' },
      { status: 500 }
    );
  }
}
