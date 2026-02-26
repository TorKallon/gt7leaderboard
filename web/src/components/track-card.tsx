import Link from 'next/link';
import { formatLapTime } from './lap-time';

export interface TrackCardData {
  slug: string;
  name: string;
  layout: string;
  country?: string | null;
  lap_count: number;
  driver_count: number;
  record_holder: string | null;
  record_time_ms: number | null;
}

export function TrackCard({ track }: { track: TrackCardData }) {
  return (
    <Link
      href={`/tracks/${track.slug}`}
      className="block rounded-lg bg-[#1f1f1f] border border-neutral-800 p-5 hover:border-neutral-600 hover:bg-[#252525] transition-all group"
    >
      <div className="mb-3">
        <h3 className="text-lg font-bold text-white group-hover:text-neutral-100 transition-colors">
          {track.name}
        </h3>
        {track.layout && (
          <p className="text-sm text-neutral-500">{track.layout}</p>
        )}
        {track.country && (
          <p className="text-xs text-neutral-600 mt-0.5">{track.country}</p>
        )}
      </div>

      <div className="flex items-center gap-4 text-xs text-neutral-400">
        <span>
          <span className="text-neutral-300 font-medium">{track.lap_count}</span>{' '}
          laps
        </span>
        <span>
          <span className="text-neutral-300 font-medium">
            {track.driver_count}
          </span>{' '}
          {track.driver_count === 1 ? 'driver' : 'drivers'}
        </span>
      </div>

      {track.record_holder && track.record_time_ms != null && (
        <div className="mt-3 pt-3 border-t border-neutral-800">
          <div className="flex items-center justify-between">
            <span className="text-xs text-neutral-500">Record</span>
            <div className="text-right">
              <span className="font-mono tabular-nums text-sm text-yellow-500 font-bold">
                {formatLapTime(track.record_time_ms)}
              </span>
              <span className="text-xs text-neutral-500 ml-2">
                {track.record_holder}
              </span>
            </div>
          </div>
        </div>
      )}
    </Link>
  );
}
