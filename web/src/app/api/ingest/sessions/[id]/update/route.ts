import { NextResponse } from 'next/server';
import { validateIngestAuth } from '@/lib/ingest-auth';
import { db } from '@/lib/db';
import { sessions, lapRecords, tracks } from '@/lib/db/schema';
import { eq } from 'drizzle-orm';

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
    const { track_slug, detection_method } = body;

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

    return NextResponse.json({ ok: true });
  } catch (error) {
    console.error('Failed to update session:', error);
    return NextResponse.json(
      { error: 'Failed to update session' },
      { status: 500 }
    );
  }
}
