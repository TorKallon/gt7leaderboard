import { NextResponse } from 'next/server';
import { validateIngestAuth } from '@/lib/ingest-auth';
import { db } from '@/lib/db';
import { sessions, tracks, cars, drivers } from '@/lib/db/schema';
import { eq } from 'drizzle-orm';

async function ensureCarExists(carId: number): Promise<void> {
  const existing = await db
    .select({ id: cars.id })
    .from(cars)
    .where(eq(cars.id, carId))
    .limit(1);
  if (existing.length === 0) {
    await db.insert(cars).values({
      id: carId,
      name: `Unknown(${carId})`,
      manufacturer: 'Unknown',
      category: 'N',
    });
  }
}

async function findOrCreateDriver(driverName: string): Promise<string> {
  // Look up by display name first.
  const existing = await db
    .select({ id: drivers.id })
    .from(drivers)
    .where(eq(drivers.displayName, driverName))
    .limit(1);
  if (existing.length > 0) {
    return existing[0].id;
  }

  // Create a new driver record.
  const result = await db
    .insert(drivers)
    .values({
      displayName: driverName,
      psnOnlineId: driverName,
    })
    .returning({ id: drivers.id });
  return result[0].id;
}

export async function POST(request: Request) {
  if (!validateIngestAuth(request)) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
  }

  try {
    const body = await request.json();
    const { driver_id, driver_name, track_slug, car_id, started_at, detection_method } = body;

    // Resolve driver: prefer driver_id (UUID), fall back to finding/creating from driver_name.
    let driverId: string | null = driver_id || null;
    if (!driverId && driver_name) {
      driverId = await findOrCreateDriver(driver_name);
    }

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

    // Ensure the car exists in the cars table before inserting the session.
    if (car_id) {
      await ensureCarExists(car_id);
    }

    const result = await db
      .insert(sessions)
      .values({
        driverId: driverId,
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
