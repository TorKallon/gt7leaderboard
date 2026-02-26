'use client';

import { useSearchParams, useRouter, usePathname } from 'next/navigation';
import { useCallback } from 'react';

const CATEGORIES = [
  { value: '', label: 'Overall' },
  { value: 'Gr.1', label: 'Gr.1' },
  { value: 'Gr.2', label: 'Gr.2' },
  { value: 'Gr.3', label: 'Gr.3' },
  { value: 'Gr.4', label: 'Gr.4' },
  { value: 'Gr.B', label: 'Gr.B' },
  { value: 'N100-300', label: 'N100-300' },
  { value: 'N300-500', label: 'N300-500' },
  { value: 'N500-700', label: 'N500-700' },
  { value: 'N700+', label: 'N700+' },
  { value: 'Gr.X', label: 'Gr.X' },
];

export function CategoryTabs() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const activeCategory = searchParams.get('category') ?? '';

  const handleClick = useCallback(
    (value: string) => {
      const params = new URLSearchParams(searchParams.toString());
      if (value) {
        params.set('category', value);
      } else {
        params.delete('category');
      }
      router.push(`${pathname}?${params.toString()}`);
    },
    [searchParams, router, pathname]
  );

  return (
    <div className="flex gap-1 overflow-x-auto pb-1 scrollbar-thin">
      {CATEGORIES.map((cat) => (
        <button
          key={cat.value}
          onClick={() => handleClick(cat.value)}
          className={`px-3 py-1.5 rounded-md text-xs font-medium whitespace-nowrap transition-colors ${
            activeCategory === cat.value
              ? 'bg-neutral-700 text-white'
              : 'text-neutral-400 hover:text-white hover:bg-neutral-800/60'
          }`}
        >
          {cat.label}
        </button>
      ))}
    </div>
  );
}
