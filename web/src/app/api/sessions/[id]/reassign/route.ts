import { NextResponse } from 'next/server';
import { revalidatePath } from 'next/cache';
import { db } from '@/lib/db';
import { sessions, lapRecords } from '@/lib/db/schema';
import { eq, isNull, and } from 'drizzle-orm';

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

    // Snapshot the current driver as auto-detected (only if not already set)
    // so the original assignment is preserved for audit.
    const sessionRows = await db
      .select({ driverId: sessions.driverId })
      .from(sessions)
      .where(eq(sessions.id, id))
      .limit(1);

    if (sessionRows.length === 0) {
      return NextResponse.json(
        { error: 'Session not found' },
        { status: 404 }
      );
    }

    const originalDriverId = sessionRows[0].driverId;

    // Save original driver to autoDetectedDriverId on laps that don't have one yet.
    if (originalDriverId) {
      await db
        .update(lapRecords)
        .set({ autoDetectedDriverId: originalDriverId, updatedAt: new Date() })
        .where(
          and(
            eq(lapRecords.sessionId, id),
            isNull(lapRecords.autoDetectedDriverId)
          )
        );
    }

    // Update session driver
    await db
      .update(sessions)
      .set({
        driverId: driver_id,
        updatedAt: new Date(),
      })
      .where(eq(sessions.id, id));

    // Update all lap records in the session
    await db
      .update(lapRecords)
      .set({
        driverId: driver_id,
        updatedAt: new Date(),
      })
      .where(eq(lapRecords.sessionId, id));

    revalidatePath('/sessions');
    revalidatePath(`/sessions/${id}`);

    return NextResponse.json({ ok: true });
  } catch (error) {
    console.error('Failed to reassign session:', error);
    return NextResponse.json(
      { error: 'Failed to reassign session' },
      { status: 500 }
    );
  }
}
