import { NextResponse } from 'next/server';
import { validateIngestAuth } from '@/lib/ingest-auth';
import { db } from '@/lib/db';
import { cars } from '@/lib/db/schema';
import { sql } from 'drizzle-orm';

export async function POST(request: Request) {
  if (!validateIngestAuth(request)) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
  }

  try {
    const body = await request.json();
    const { cars: carList } = body;

    if (!Array.isArray(carList) || carList.length === 0) {
      return NextResponse.json(
        { error: 'cars array is required' },
        { status: 400 }
      );
    }

    let synced = 0;
    for (const car of carList) {
      await db
        .insert(cars)
        .values({
          id: car.id,
          name: car.name,
          manufacturer: car.manufacturer,
          category: car.category,
          ppStock: car.pp_stock ?? null,
        })
        .onConflictDoUpdate({
          target: cars.id,
          set: {
            name: sql`excluded.name`,
            manufacturer: sql`excluded.manufacturer`,
            category: sql`excluded.category`,
            ppStock: sql`excluded.pp_stock`,
            updatedAt: new Date(),
          },
        });
      synced++;
    }

    return NextResponse.json({ synced });
  } catch (error) {
    console.error('Failed to sync cars:', error);
    return NextResponse.json(
      { error: 'Failed to sync cars' },
      { status: 500 }
    );
  }
}
