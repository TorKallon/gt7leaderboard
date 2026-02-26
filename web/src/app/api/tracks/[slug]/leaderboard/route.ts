import { NextResponse } from 'next/server';
import { db } from '@/lib/db';
import { tracks } from '@/lib/db/schema';
import { getTrackLeaderboard } from '@/lib/db/queries';
import { eq } from 'drizzle-orm';

export async function GET(
  request: Request,
  { params }: { params: Promise<{ slug: string }> }
) {
  try {
    const { slug } = await params;
    const url = new URL(request.url);
    const category = url.searchParams.get('category') ?? undefined;
    const carIdParam = url.searchParams.get('car_id');
    const carId = carIdParam ? parseInt(carIdParam, 10) : undefined;

    // Look up track by slug
    const trackRows = await db
      .select({ id: tracks.id })
      .from(tracks)
      .where(eq(tracks.slug, slug))
      .limit(1);

    if (trackRows.length === 0) {
      return NextResponse.json(
        { error: 'Track not found' },
        { status: 404 }
      );
    }

    const leaderboard = await getTrackLeaderboard(db, trackRows[0].id, {
      category,
      carId,
    });

    return NextResponse.json({ leaderboard });
  } catch (error) {
    console.error('Failed to get leaderboard:', error);
    return NextResponse.json(
      { error: 'Failed to get leaderboard' },
      { status: 500 }
    );
  }
}
