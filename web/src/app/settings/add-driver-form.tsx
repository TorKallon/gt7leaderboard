'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';

export function AddDriverForm() {
  const router = useRouter();
  const [name, setName] = useState('');
  const [psnId, setPsnId] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setSuccess('');

    if (!name.trim()) {
      setError('Name is required');
      return;
    }

    setIsSubmitting(true);

    try {
      const res = await fetch('/api/drivers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          display_name: name.trim(),
          psn_online_id: psnId.trim() || undefined,
        }),
      });

      if (!res.ok) {
        throw new Error('Failed to create driver');
      }

      setName('');
      setPsnId('');
      setSuccess(`Driver "${name.trim()}" created successfully.`);
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong');
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-3">
      {error && (
        <div className="rounded-md bg-red-900/30 border border-red-700/40 px-3 py-2 text-sm text-red-300">
          {error}
        </div>
      )}
      {success && (
        <div className="rounded-md bg-green-900/30 border border-green-700/40 px-3 py-2 text-sm text-green-300">
          {success}
        </div>
      )}

      <div className="flex flex-col sm:flex-row gap-3">
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Display name *"
          className="flex-1 rounded-md bg-neutral-900 border border-neutral-700 text-white text-sm px-3 py-2 focus:outline-none focus:ring-1 focus:ring-neutral-500 placeholder:text-neutral-600"
        />
        <input
          type="text"
          value={psnId}
          onChange={(e) => setPsnId(e.target.value)}
          placeholder="PSN Online ID (optional)"
          className="flex-1 rounded-md bg-neutral-900 border border-neutral-700 text-white text-sm px-3 py-2 focus:outline-none focus:ring-1 focus:ring-neutral-500 placeholder:text-neutral-600"
        />
        <button
          type="submit"
          disabled={isSubmitting}
          className="px-4 py-2 rounded-md bg-neutral-700 text-sm font-medium text-white hover:bg-neutral-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
        >
          {isSubmitting ? 'Adding...' : 'Add Driver'}
        </button>
      </div>
    </form>
  );
}
