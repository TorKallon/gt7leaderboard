'use client';

import { useRouter } from 'next/navigation';

export function SessionLoadMore({ initialCount }: { initialCount: number }) {
  const router = useRouter();

  // Simple approach: reload the page (server-side pagination would be more complex)
  // For now, this is a placeholder that can be enhanced later
  return (
    <button
      onClick={() => router.refresh()}
      className="px-4 py-2 rounded-md bg-neutral-800 text-sm text-neutral-300 hover:bg-neutral-700 hover:text-white transition-colors border border-neutral-700"
    >
      Showing {initialCount} sessions. Reload for latest.
    </button>
  );
}
