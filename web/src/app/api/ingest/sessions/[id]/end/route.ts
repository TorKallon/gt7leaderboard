import { NextResponse } from 'next/server';
import { validateIngestAuth } from '@/lib/ingest-auth';
import { db } from '@/lib/db';
import { sessions } from '@/lib/db/schema';
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
    const { ended_at } = body;

    await db
      .update(sessions)
      .set({
        endedAt: new Date(ended_at),
        updatedAt: new Date(),
      })
      .where(eq(sessions.id, id));

    return NextResponse.json({ ok: true });
  } catch (error) {
    console.error('Failed to end session:', error);
    return NextResponse.json(
      { error: 'Failed to end session' },
      { status: 500 }
    );
  }
}
