/**
 * Colored badge that displays a driver name.
 * Color is deterministically derived from the name so each driver always gets the same color.
 */

const BADGE_COLORS = [
  'bg-red-900/60 text-red-300 border-red-700/40',
  'bg-blue-900/60 text-blue-300 border-blue-700/40',
  'bg-green-900/60 text-green-300 border-green-700/40',
  'bg-purple-900/60 text-purple-300 border-purple-700/40',
  'bg-orange-900/60 text-orange-300 border-orange-700/40',
  'bg-cyan-900/60 text-cyan-300 border-cyan-700/40',
  'bg-pink-900/60 text-pink-300 border-pink-700/40',
  'bg-yellow-900/60 text-yellow-300 border-yellow-700/40',
];

function hashName(name: string): number {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = (hash << 5) - hash + name.charCodeAt(i);
    hash |= 0;
  }
  return Math.abs(hash);
}

export function DriverBadge({
  name,
  className,
}: {
  name: string;
  className?: string;
}) {
  const colorClass = BADGE_COLORS[hashName(name) % BADGE_COLORS.length];

  return (
    <span
      className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium border ${colorClass} ${className ?? ''}`}
    >
      {name}
    </span>
  );
}
