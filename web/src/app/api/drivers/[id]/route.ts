import { NextResponse } from 'next/server';
import { db } from '@/lib/db';
import { drivers } from '@/lib/db/schema';
import { eq } from 'drizzle-orm';
import { getDriverStats } from '@/lib/db/queries';

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;

    const driverRows = await db
      .select()
      .from(drivers)
      .where(eq(drivers.id, id))
      .limit(1);

    if (driverRows.length === 0) {
      return NextResponse.json(
        { error: 'Driver not found' },
        { status: 404 }
      );
    }

    const stats = await getDriverStats(db, id);

    return NextResponse.json({
      driver: driverRows[0],
      stats,
    });
  } catch (error) {
    console.error('Failed to get driver:', error);
    return NextResponse.json(
      { error: 'Failed to get driver' },
      { status: 500 }
    );
  }
}

export async function PATCH(
  request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;

    // Only allow editing guest drivers
    const existing = await db
      .select({ isGuest: drivers.isGuest })
      .from(drivers)
      .where(eq(drivers.id, id))
      .limit(1);

    if (existing.length === 0) {
      return NextResponse.json(
        { error: 'Driver not found' },
        { status: 404 }
      );
    }

    if (!existing[0].isGuest) {
      return NextResponse.json(
        { error: 'Only guest drivers can be edited' },
        { status: 403 }
      );
    }

    const body = await request.json();
    const { display_name, psn_online_id } = body;

    const updates: Record<string, unknown> = {
      updatedAt: new Date(),
    };

    if (display_name !== undefined) {
      updates.displayName = display_name;
    }

    if (psn_online_id !== undefined) {
      updates.psnOnlineId = psn_online_id;
    }

    const result = await db
      .update(drivers)
      .set(updates)
      .where(eq(drivers.id, id))
      .returning();

    if (result.length === 0) {
      return NextResponse.json(
        { error: 'Driver not found' },
        { status: 404 }
      );
    }

    return NextResponse.json({ driver: result[0] });
  } catch (error) {
    console.error('Failed to update driver:', error);
    return NextResponse.json(
      { error: 'Failed to update driver' },
      { status: 500 }
    );
  }
}
