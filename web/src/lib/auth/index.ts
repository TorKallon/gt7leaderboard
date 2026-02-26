import NextAuth from 'next-auth';
import Google from 'next-auth/providers/google';

const providers = [];
if (process.env.GOOGLE_CLIENT_ID && process.env.GOOGLE_CLIENT_SECRET) {
  providers.push(
    Google({
      clientId: process.env.GOOGLE_CLIENT_ID,
      clientSecret: process.env.GOOGLE_CLIENT_SECRET,
    })
  );
}

const allowedDomain = process.env.ALLOWED_DOMAIN || '';
const allowedEmails = (process.env.ALLOWED_EMAILS || '')
  .split(',')
  .map((e) => e.trim().toLowerCase())
  .filter(Boolean);

export const { auth, handlers, signIn, signOut } = NextAuth({
  providers,
  callbacks: {
    signIn({ profile }) {
      const email = profile?.email?.toLowerCase() || '';

      // If no restrictions configured, allow all
      if (!allowedDomain && allowedEmails.length === 0) return true;

      // Check explicit email allowlist
      if (allowedEmails.includes(email)) return true;

      // Check domain
      if (allowedDomain && email.endsWith(`@${allowedDomain}`)) return true;

      return false;
    },
  },
});
