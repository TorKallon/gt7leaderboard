/**
 * Lap time formatting and display components.
 */

export function formatLapTime(ms: number): string {
  if (ms <= 0) return '0:00.000';

  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  const millis = ms % 1000;

  return `${minutes}:${seconds.toString().padStart(2, '0')}.${millis
    .toString()
    .padStart(3, '0')}`;
}

export function LapTime({ ms, className }: { ms: number; className?: string }) {
  return (
    <span className={`font-mono tabular-nums ${className ?? ''}`}>
      {formatLapTime(ms)}
    </span>
  );
}

export function LapTimeDelta({
  deltaMs,
  className,
}: {
  deltaMs: number;
  className?: string;
}) {
  if (deltaMs === 0) {
    return (
      <span className={`font-mono tabular-nums text-neutral-500 ${className ?? ''}`}>
        --
      </span>
    );
  }

  const prefix = deltaMs > 0 ? '+' : '-';
  const absMs = Math.abs(deltaMs);
  const color = deltaMs < 0 ? 'text-green-500' : 'text-red-500';

  return (
    <span className={`font-mono tabular-nums ${color} ${className ?? ''}`}>
      {prefix}{formatLapTime(absMs)}
    </span>
  );
}
