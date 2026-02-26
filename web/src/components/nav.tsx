'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

const NAV_LINKS = [
  { href: '/', label: 'Dashboard' },
  { href: '/tracks', label: 'Tracks' },
  { href: '/drivers', label: 'Drivers' },
  { href: '/sessions', label: 'Sessions' },
  { href: '/settings', label: 'Settings' },
];

export function Nav() {
  const pathname = usePathname();

  return (
    <nav className="sticky top-0 z-50 border-b border-neutral-800 bg-[#0f0f0f]/95 backdrop-blur-sm">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="flex h-14 items-center gap-8">
          <Link href="/" className="flex items-center gap-2 shrink-0">
            <span className="text-lg font-bold text-white tracking-tight">
              GT7
            </span>
            <span className="text-lg font-light text-neutral-400 tracking-tight">
              Leaderboard
            </span>
          </Link>

          <div className="flex items-center gap-1 overflow-x-auto">
            {NAV_LINKS.map((link) => {
              const isActive =
                link.href === '/'
                  ? pathname === '/'
                  : pathname.startsWith(link.href);

              return (
                <Link
                  key={link.href}
                  href={link.href}
                  className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors whitespace-nowrap ${
                    isActive
                      ? 'bg-neutral-800 text-white'
                      : 'text-neutral-400 hover:text-white hover:bg-neutral-800/50'
                  }`}
                >
                  {link.label}
                </Link>
              );
            })}
          </div>
        </div>
      </div>
    </nav>
  );
}
