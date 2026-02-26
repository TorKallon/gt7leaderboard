import { NextResponse } from 'next/server';
import { validateIngestAuth } from '@/lib/ingest-auth';
import { db } from '@/lib/db';
import { lapRecords, sessions } from '@/lib/db/schema';
import { eq } from 'drizzle-orm';
import { checkForRecords } from '@/lib/db/queries';

export async function POST(request: Request) {
  if (!validateIngestAuth(request)) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
  }

  try {
    const body = await request.json();
    const { session_id, lap_time_ms, lap_number, recorded_at } = body;

    // Look up session to get driver_id, track_id, car_id
    const sessionRows = await db
      .select({
        driverId: sessions.driverId,
        trackId: sessions.trackId,
        carId: sessions.carId,
      })
      .from(sessions)
      .where(eq(sessions.id, session_id))
      .limit(1);

    if (sessionRows.length === 0) {
      return NextResponse.json(
        { error: 'Session not found' },
        { status: 404 }
      );
    }

    const session = sessionRows[0];

    // Check for records BEFORE inserting so the new lap doesn't affect the query.
    let records: Array<{
      type: string;
      previous_time_ms: number | null;
      previous_driver: string | null;
    }> = [];

    if (session.trackId && session.carId) {
      const rawRecords = await checkForRecords(
        db,
        session.trackId,
        session.carId,
        lap_time_ms
      );
      records = rawRecords.map((r) => ({
        type: r.type,
        previous_time_ms: r.previousTimeMs,
        previous_driver: r.previousDriver,
      }));
    }

    const result = await db
      .insert(lapRecords)
      .values({
        sessionId: session_id,
        driverId: session.driverId,
        trackId: session.trackId,
        carId: session.carId,
        lapTimeMs: lap_time_ms,
        lapNumber: lap_number,
        recordedAt: new Date(recorded_at),
      })
      .returning({ id: lapRecords.id });

    return NextResponse.json({
      lap_id: result[0].id,
      records,
    });
  } catch (error) {
    console.error('Failed to record lap:', error);
    return NextResponse.json(
      { error: 'Failed to record lap' },
      { status: 500 }
    );
  }
}
