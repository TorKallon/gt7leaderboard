import { NextResponse } from 'next/server';
import { db } from '@/lib/db';
import { sessions, lapRecords } from '@/lib/db/schema';
import { eq } from 'drizzle-orm';

export async function PATCH(
  request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const body = await request.json();
    const { driver_id } = body;

    if (!driver_id) {
      return NextResponse.json(
        { error: 'driver_id is required' },
        { status: 400 }
      );
    }

    // Update session driver
    const sessionResult = await db
      .update(sessions)
      .set({
        driverId: driver_id,
        updatedAt: new Date(),
      })
      .where(eq(sessions.id, id))
      .returning({ id: sessions.id });

    if (sessionResult.length === 0) {
      return NextResponse.json(
        { error: 'Session not found' },
        { status: 404 }
      );
    }

    // Update all lap records in the session (preserve auto_detected_driver_id)
    await db
      .update(lapRecords)
      .set({
        driverId: driver_id,
        updatedAt: new Date(),
      })
      .where(eq(lapRecords.sessionId, id));

    return NextResponse.json({ ok: true });
  } catch (error) {
    console.error('Failed to reassign session:', error);
    return NextResponse.json(
      { error: 'Failed to reassign session' },
      { status: 500 }
    );
  }
}
