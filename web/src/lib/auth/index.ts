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

export const { auth, handlers, signIn, signOut } = NextAuth({
  providers,
  callbacks: {
    signIn({ profile }) {
      // If no domain restriction configured, allow all
      if (!allowedDomain) return true;
      if (!profile?.email?.endsWith(`@${allowedDomain}`)) {
        return false;
      }
      return true;
    },
  },
});
