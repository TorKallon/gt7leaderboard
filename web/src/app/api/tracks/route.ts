import { NextResponse } from 'next/server';
import { db } from '@/lib/db';
import { getTrackList } from '@/lib/db/queries';

export async function GET() {
  try {
    const trackList = await getTrackList(db);
    return NextResponse.json({ tracks: trackList });
  } catch (error) {
    console.error('Failed to get tracks:', error);
    return NextResponse.json(
      { error: 'Failed to get tracks' },
      { status: 500 }
    );
  }
}
