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
