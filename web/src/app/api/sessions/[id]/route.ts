import { NextResponse } from 'next/server';
import { db } from '@/lib/db';
import { sessions, lapRecords } from '@/lib/db/schema';
import { sql, eq } from 'drizzle-orm';

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;

    // Get session with driver/track/car info
    const sessionResult = await db.execute(sql`
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
        s.created_at
      FROM sessions s
      LEFT JOIN drivers d ON d.id = s.driver_id
      LEFT JOIN tracks t ON t.id = s.track_id
      LEFT JOIN cars c ON c.id = s.car_id
      WHERE s.id = ${id}
    `);

    if (sessionResult.rows.length === 0) {
      return NextResponse.json(
        { error: 'Session not found' },
        { status: 404 }
      );
    }

    // Get all laps for the session
    const lapsResult = await db.execute(sql`
      SELECT
        lr.id,
        lr.lap_time_ms,
        lr.lap_number,
        lr.driver_id,
        d.display_name AS driver_name,
        lr.auto_detected_driver_id,
        lr.weather,
        lr.is_valid,
        lr.recorded_at
      FROM lap_records lr
      LEFT JOIN drivers d ON d.id = lr.driver_id
      WHERE lr.session_id = ${id}
      ORDER BY lr.lap_number ASC
    `);

    return NextResponse.json({
      session: sessionResult.rows[0],
      laps: lapsResult.rows,
    });
  } catch (error) {
    console.error('Failed to get session:', error);
    return NextResponse.json(
      { error: 'Failed to get session' },
      { status: 500 }
    );
  }
}

export async function DELETE(
  _request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;

    // Delete child lap records first (no cascade on FK)
    await db.delete(lapRecords).where(eq(lapRecords.sessionId, id));

    const result = await db
      .delete(sessions)
      .where(eq(sessions.id, id))
      .returning({ id: sessions.id });

    if (result.length === 0) {
      return NextResponse.json(
        { error: 'Session not found' },
        { status: 404 }
      );
    }

    return NextResponse.json({ deleted: true });
  } catch (error) {
    console.error('Failed to delete session:', error);
    return NextResponse.json(
      { error: 'Failed to delete session' },
      { status: 500 }
    );
  }
}
