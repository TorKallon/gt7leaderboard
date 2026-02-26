'use client';

import { useSearchParams, useRouter, usePathname } from 'next/navigation';

interface CarOption {
  id: number;
  name: string;
  manufacturer: string;
}

export function CarFilter({ cars }: { cars: CarOption[] }) {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const activeCarId = searchParams.get('car_id') ?? '';

  function handleChange(value: string) {
    const params = new URLSearchParams(searchParams.toString());
    if (value) {
      params.set('car_id', value);
    } else {
      params.delete('car_id');
    }
    router.push(`${pathname}?${params.toString()}`);
  }

  if (cars.length === 0) return null;

  return (
    <select
      value={activeCarId}
      onChange={(e) => handleChange(e.target.value)}
      className="rounded-md bg-neutral-900 border border-neutral-700 text-white text-xs px-3 py-1.5 focus:outline-none focus:ring-1 focus:ring-neutral-500"
    >
      <option value="">All Cars</option>
      {cars.map((car) => (
        <option key={car.id} value={car.id}>
          {car.manufacturer} {car.name}
        </option>
      ))}
    </select>
  );
}
