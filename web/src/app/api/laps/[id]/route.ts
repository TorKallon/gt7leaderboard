import { NextResponse } from 'next/server';
import { db } from '@/lib/db';
import { lapRecords } from '@/lib/db/schema';
import { eq } from 'drizzle-orm';

export async function PATCH(
  request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const body = await request.json();
    const { driver_id, weather } = body;

    const updates: Record<string, unknown> = {
      updatedAt: new Date(),
    };

    if (driver_id !== undefined) {
      updates.driverId = driver_id;
    }
    if (weather !== undefined) {
      updates.weather = weather;
    }

    const result = await db
      .update(lapRecords)
      .set(updates)
      .where(eq(lapRecords.id, id))
      .returning();

    if (result.length === 0) {
      return NextResponse.json(
        { error: 'Lap not found' },
        { status: 404 }
      );
    }

    return NextResponse.json({ lap: result[0] });
  } catch (error) {
    console.error('Failed to update lap:', error);
    return NextResponse.json(
      { error: 'Failed to update lap' },
      { status: 500 }
    );
  }
}
