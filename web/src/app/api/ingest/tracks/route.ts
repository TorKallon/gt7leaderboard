import { NextResponse } from 'next/server';
import { validateIngestAuth } from '@/lib/ingest-auth';
import { db } from '@/lib/db';
import { tracks } from '@/lib/db/schema';
import { sql } from 'drizzle-orm';

export async function POST(request: Request) {
  if (!validateIngestAuth(request)) {
    return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
  }

  try {
    const body = await request.json();
    const { name, layout, slug, country, length_meters, num_corners, has_weather } = body;

    if (!name || !slug) {
      return NextResponse.json(
        { error: 'name and slug are required' },
        { status: 400 }
      );
    }

    const result = await db
      .insert(tracks)
      .values({
        name,
        layout: layout ?? '',
        slug,
        country: country ?? null,
        lengthMeters: length_meters ?? null,
        numCorners: num_corners ?? null,
        hasWeather: has_weather ?? false,
      })
      .onConflictDoUpdate({
        target: tracks.slug,
        set: {
          name: sql`excluded.name`,
          layout: sql`excluded.layout`,
          country: sql`excluded.country`,
          lengthMeters: sql`excluded.length_meters`,
          numCorners: sql`excluded.num_corners`,
          hasWeather: sql`excluded.has_weather`,
        },
      })
      .returning({ id: tracks.id });

    return NextResponse.json({ track_id: result[0].id });
  } catch (error) {
    console.error('Failed to sync track:', error);
    return NextResponse.json(
      { error: 'Failed to sync track' },
      { status: 500 }
    );
  }
}
