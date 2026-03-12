'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';

export function DeleteSessionDialog({ sessionId }: { sessionId: string }) {
  const router = useRouter();
  const [isOpen, setIsOpen] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [error, setError] = useState('');

  async function handleDelete() {
    setError('');
    setIsDeleting(true);

    try {
      const res = await fetch(`/api/sessions/${sessionId}`, {
        method: 'DELETE',
      });

      if (!res.ok) {
        throw new Error('Failed to delete session');
      }

      router.push('/sessions');
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong');
      setIsDeleting(false);
    }
  }

  return (
    <>
      <button
        onClick={() => setIsOpen(true)}
        className="px-4 py-2 rounded-md bg-neutral-800 text-sm font-medium text-red-400 hover:bg-neutral-700 hover:text-red-300 transition-colors border border-neutral-700"
      >
        Delete Session
      </button>

      {isOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div
            className="fixed inset-0 bg-black/60"
            onClick={() => setIsOpen(false)}
          />
          <div className="relative w-full max-w-sm rounded-lg bg-[#1f1f1f] border border-neutral-700 p-6 shadow-xl">
            <h2 className="text-lg font-bold text-white mb-2">
              Delete Session
            </h2>
            <p className="text-sm text-neutral-400 mb-4">
              This will permanently delete this session and all its lap records.
              This cannot be undone.
            </p>

            {error && (
              <div className="mb-4 rounded-md bg-red-900/30 border border-red-700/40 px-3 py-2 text-sm text-red-300">
                {error}
              </div>
            )}

            <div className="flex justify-end gap-3">
              <button
                onClick={() => setIsOpen(false)}
                className="px-4 py-2 rounded-md text-sm text-neutral-400 hover:text-white transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleDelete}
                disabled={isDeleting}
                className="px-4 py-2 rounded-md bg-red-600 text-sm font-medium text-white hover:bg-red-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isDeleting ? 'Deleting...' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
