'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';

export function EditDriverDialog({
  driverId,
  initialName,
  initialPsnOnlineId,
}: {
  driverId: string;
  initialName: string;
  initialPsnOnlineId: string | null;
}) {
  const router = useRouter();
  const [isOpen, setIsOpen] = useState(false);
  const [displayName, setDisplayName] = useState(initialName);
  const [psnOnlineId, setPsnOnlineId] = useState(initialPsnOnlineId ?? '');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState('');

  function handleOpen() {
    setDisplayName(initialName);
    setPsnOnlineId(initialPsnOnlineId ?? '');
    setError('');
    setIsOpen(true);
  }

  async function handleSubmit() {
    setError('');

    if (!displayName.trim()) {
      setError('Display name is required');
      return;
    }

    setIsSubmitting(true);

    try {
      const res = await fetch(`/api/drivers/${driverId}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          display_name: displayName.trim(),
          psn_online_id: psnOnlineId.trim() || null,
        }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || 'Failed to update driver');
      }

      setIsOpen(false);
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong');
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <>
      <button
        onClick={handleOpen}
        className="text-xs text-neutral-500 hover:text-neutral-300 transition-colors"
      >
        Edit
      </button>

      {isOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div
            className="fixed inset-0 bg-black/60"
            onClick={() => setIsOpen(false)}
          />
          <div className="relative w-full max-w-md rounded-lg bg-[#1f1f1f] border border-neutral-700 p-6 shadow-xl">
            <h2 className="text-lg font-bold text-white mb-4">
              Edit Guest Driver
            </h2>

            {error && (
              <div className="mb-4 rounded-md bg-red-900/30 border border-red-700/40 px-3 py-2 text-sm text-red-300">
                {error}
              </div>
            )}

            <label className="block text-sm text-neutral-400 mb-2">
              Display Name
            </label>
            <input
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              className="w-full rounded-md bg-neutral-900 border border-neutral-700 text-white text-sm px-3 py-2 mb-4 focus:outline-none focus:ring-1 focus:ring-neutral-500"
            />

            <label className="block text-sm text-neutral-400 mb-2">
              PSN Online ID
            </label>
            <input
              type="text"
              value={psnOnlineId}
              onChange={(e) => setPsnOnlineId(e.target.value)}
              placeholder="Optional"
              className="w-full rounded-md bg-neutral-900 border border-neutral-700 text-white text-sm px-3 py-2 mb-4 focus:outline-none focus:ring-1 focus:ring-neutral-500"
            />

            <div className="flex justify-end gap-3 mt-2">
              <button
                onClick={() => setIsOpen(false)}
                className="px-4 py-2 rounded-md text-sm text-neutral-400 hover:text-white transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleSubmit}
                disabled={isSubmitting}
                className="px-4 py-2 rounded-md bg-red-600 text-sm font-medium text-white hover:bg-red-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isSubmitting ? 'Saving...' : 'Save'}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
