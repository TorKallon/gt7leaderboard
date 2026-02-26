'use client';

import { useState } from 'react';

const WEATHER_OPTIONS = ['unknown', 'dry', 'wet'] as const;

const WEATHER_LABELS: Record<string, string> = {
  unknown: 'Unknown',
  dry: 'Dry',
  wet: 'Wet',
};

const WEATHER_COLORS: Record<string, string> = {
  unknown: 'text-neutral-400',
  dry: 'text-yellow-400',
  wet: 'text-blue-400',
};

export function WeatherTag({
  lapId,
  initialWeather,
}: {
  lapId: string;
  initialWeather: string;
}) {
  const [weather, setWeather] = useState(initialWeather);
  const [isSaving, setIsSaving] = useState(false);

  async function handleChange(value: string) {
    setIsSaving(true);
    setWeather(value);

    try {
      await fetch(`/api/laps/${lapId}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ weather: value }),
      });
    } catch {
      // Revert on error
      setWeather(initialWeather);
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <select
      value={weather}
      onChange={(e) => handleChange(e.target.value)}
      disabled={isSaving}
      className={`bg-transparent border border-neutral-700 rounded text-xs px-1.5 py-0.5 focus:outline-none focus:ring-1 focus:ring-neutral-500 ${WEATHER_COLORS[weather] ?? 'text-neutral-400'} disabled:opacity-50`}
    >
      {WEATHER_OPTIONS.map((w) => (
        <option key={w} value={w} className="bg-neutral-900 text-white">
          {WEATHER_LABELS[w]}
        </option>
      ))}
    </select>
  );
}
