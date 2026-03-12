import { auth } from '@/lib/auth';
import { NextResponse } from 'next/server';

export default auth((req) => {
  const { pathname } = req.nextUrl;

  // Allow ingest endpoints (they use API key auth)
  if (pathname.startsWith('/api/ingest')) {
    return NextResponse.next();
  }

  // Allow auth endpoints
  if (pathname.startsWith('/api/auth')) {
    return NextResponse.next();
  }

  // Allow public pages: leaderboard, track pages, driver pages
  if (
    pathname === '/' ||
    pathname.startsWith('/tracks') ||
    pathname.startsWith('/drivers') ||
    pathname.startsWith('/api/tracks') ||
    pathname.startsWith('/api/drivers') ||
    pathname === '/api/status'
  ) {
    return NextResponse.next();
  }

  // In dev without Google OAuth configured, skip authentication
  if (!process.env.GOOGLE_CLIENT_ID) {
    return NextResponse.next();
  }

  // Redirect unauthenticated users to sign-in
  if (!req.auth) {
    const signInUrl = new URL('/api/auth/signin', req.nextUrl.origin);
    signInUrl.searchParams.set('callbackUrl', req.nextUrl.href);
    return NextResponse.redirect(signInUrl);
  }

  return NextResponse.next();
});

export const config = {
  matcher: [
    // Match all paths except static files and _next
    '/((?!_next/static|_next/image|favicon.ico).*)',
  ],
};
