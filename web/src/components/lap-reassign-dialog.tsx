'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';

interface Driver {
  id: string;
  display_name: string;
  is_guest: boolean;
}

export function LapReassignDialog({
  lapId,
  currentDriverName,
  currentDriverId,
}: {
  lapId: string;
  currentDriverName: string | null;
  currentDriverId: string | null;
}) {
  const router = useRouter();
  const [isOpen, setIsOpen] = useState(false);
  const [drivers, setDrivers] = useState<Driver[]>([]);
  const [selectedDriverId, setSelectedDriverId] = useState(
    currentDriverId ?? ''
  );
  const [newGuestName, setNewGuestName] = useState('');
  const [isCreatingGuest, setIsCreatingGuest] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (isOpen) {
      fetch('/api/drivers')
        .then((res) => res.json())
        .then((data) => {
          if (data.drivers) {
            setDrivers(data.drivers);
          }
        })
        .catch(() => {
          setError('Failed to load drivers');
        });
    }
  }, [isOpen]);

  async function handleSubmit() {
    setError('');
    setIsSubmitting(true);

    try {
      let driverId = selectedDriverId;

      if (isCreatingGuest) {
        if (!newGuestName.trim()) {
          setError('Guest name is required');
          setIsSubmitting(false);
          return;
        }

        const createRes = await fetch('/api/drivers', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ display_name: newGuestName.trim() }),
        });

        if (!createRes.ok) {
          throw new Error('Failed to create guest driver');
        }

        const createData = await createRes.json();
        driverId = createData.driver.id;
      }

      if (!driverId) {
        setError('Please select a driver');
        setIsSubmitting(false);
        return;
      }

      const res = await fetch(`/api/laps/${lapId}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ driver_id: driverId }),
      });

      if (!res.ok) {
        throw new Error('Failed to reassign lap');
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
        onClick={() => setIsOpen(true)}
        className="text-xs text-neutral-500 hover:text-neutral-300 transition-colors"
        title="Reassign driver for this lap"
      >
        {currentDriverName ?? 'Unknown'}
        <span className="ml-1 text-neutral-600 hover:text-neutral-400">&#9998;</span>
      </button>

      {isOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div
            className="fixed inset-0 bg-black/60"
            onClick={() => setIsOpen(false)}
          />
          <div className="relative w-full max-w-md rounded-lg bg-[#1f1f1f] border border-neutral-700 p-6 shadow-xl">
            <h2 className="text-lg font-bold text-white mb-4">
              Reassign Lap Driver
            </h2>

            {error && (
              <div className="mb-4 rounded-md bg-red-900/30 border border-red-700/40 px-3 py-2 text-sm text-red-300">
                {error}
              </div>
            )}

            {!isCreatingGuest ? (
              <>
                <label className="block text-sm text-neutral-400 mb-2">
                  Select Driver
                </label>
                <select
                  value={selectedDriverId}
                  onChange={(e) => setSelectedDriverId(e.target.value)}
                  className="w-full rounded-md bg-neutral-900 border border-neutral-700 text-white text-sm px-3 py-2 mb-3 focus:outline-none focus:ring-1 focus:ring-neutral-500"
                >
                  <option value="">-- Select --</option>
                  {drivers.map((d) => (
                    <option key={d.id} value={d.id}>
                      {d.display_name} {d.is_guest ? '(Guest)' : ''}
                    </option>
                  ))}
                </select>

                <button
                  onClick={() => setIsCreatingGuest(true)}
                  className="text-xs text-neutral-500 hover:text-neutral-300 transition-colors"
                >
                  + Create Guest Driver
                </button>
              </>
            ) : (
              <>
                <label className="block text-sm text-neutral-400 mb-2">
                  Guest Driver Name
                </label>
                <input
                  type="text"
                  value={newGuestName}
                  onChange={(e) => setNewGuestName(e.target.value)}
                  placeholder="Enter name..."
                  className="w-full rounded-md bg-neutral-900 border border-neutral-700 text-white text-sm px-3 py-2 mb-3 focus:outline-none focus:ring-1 focus:ring-neutral-500"
                />
                <button
                  onClick={() => setIsCreatingGuest(false)}
                  className="text-xs text-neutral-500 hover:text-neutral-300 transition-colors"
                >
                  Back to driver list
                </button>
              </>
            )}

            <div className="flex justify-end gap-3 mt-6">
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
                {isSubmitting ? 'Saving...' : 'Reassign'}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
