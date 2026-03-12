import { NextResponse } from 'next/server';
import { validateIngestAuth } from '@/lib/ingest-auth';
import { db } from '@/lib/db';
import { sessions, lapRecords, tracks, drivers } from '@/lib/db/schema';
import { eq } from 'drizzle-orm';

async function findOrCreateDriver(psnAccountId: string | null, driverName: string): Promise<string> {
  if (psnAccountId) {
    const byAccountId = await db
      .select({ id: drivers.id })
      .from(drivers)
      .where(eq(drivers.psnAccountId, psnAccountId))
      .limit(1);
    if (byAccountId.length > 0) {
      return byAccountId[0].id;
    }
  }

  const byName = await db
    .select({ id: drivers.id, psnAccountId: drivers.psnAccountId })
    .from(drivers)
    .where(eq(drivers.displayName, driverName))
    .limit(1);
  if (byName.length > 0) {
    if (psnAccountId && !byName[0].psnAccountId) {
      await db
        .update(drivers)
        .set({ psnAccountId })
        .where(eq(drivers.id, byName[0].id));
    }
    return byName[0].id;
  }

  const result = await db
    .insert(drivers)
    .values({
      displayName: driverName,
      psnOnlineId: driverName,
      psnAccountId: psnAccountId,
    })
    .returning({ id: drivers.id });
  return result[0].id;
}

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  if (!validateIngestAuth(request)) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
  }

  try {
    const { id } = await params;
    const body = await request.json();
    const { track_slug, detection_method, driver_id, driver_name } = body;

    const updates: Record<string, unknown> = {
      updatedAt: new Date(),
    };

    let trackId: string | null = null;

    if (track_slug) {
      const trackRows = await db
        .select({ id: tracks.id })
        .from(tracks)
        .where(eq(tracks.slug, track_slug))
        .limit(1);
      if (trackRows.length > 0) {
        trackId = trackRows[0].id;
        updates.trackId = trackId;
      }
    }

    if (detection_method) {
      updates.detectionMethod = detection_method;
    }

    // Resolve driver if provided (e.g. re-detection after PSN auth).
    let resolvedDriverId: string | null = null;
    if (driver_name) {
      resolvedDriverId = await findOrCreateDriver(driver_id || null, driver_name);
      updates.driverId = resolvedDriverId;
    }

    await db
      .update(sessions)
      .set(updates)
      .where(eq(sessions.id, id));

    // Also update any existing lap records for this session with the track.
    if (trackId) {
      await db
        .update(lapRecords)
        .set({ trackId, updatedAt: new Date() })
        .where(eq(lapRecords.sessionId, id));
    }

    // Also update any existing lap records for this session with the driver.
    if (resolvedDriverId) {
      await db
        .update(lapRecords)
        .set({ driverId: resolvedDriverId, updatedAt: new Date() })
        .where(eq(lapRecords.sessionId, id));
    }

    return NextResponse.json({ ok: true });
  } catch (error) {
    console.error('Failed to update session:', error);
    return NextResponse.json(
      { error: 'Failed to update session' },
      { status: 500 }
    );
  }
}
