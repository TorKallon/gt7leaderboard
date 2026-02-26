import { neon } from '@neondatabase/serverless';
import { drizzle as drizzleNeon, NeonHttpDatabase } from 'drizzle-orm/neon-http';
import * as schema from './schema';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
let _db: any = null;

function getDb() {
  if (_db) return _db;

  const url = process.env.DATABASE_URL;
  if (!url) throw new Error('DATABASE_URL is required');

  // Auto-detect: use standard pg driver for non-Neon databases (Docker / local dev)
  if (!url.includes('neon.tech')) {
    try {
      // eslint-disable-next-line @typescript-eslint/no-require-imports
      const pg = require('pg');
      // eslint-disable-next-line @typescript-eslint/no-require-imports
      const nodePg = require('drizzle-orm/node-postgres');
      _db = nodePg.drizzle(new pg.Pool({ connectionString: url }), { schema });
      return _db;
    } catch {
      // pg not available, fall through to Neon driver
    }
  }

  // Production: Neon serverless HTTP driver
  const sql = neon(url);
  _db = drizzleNeon(sql, { schema });
  return _db;
}

// Proxy so the db connection is lazy — only created when first accessed.
// This avoids runtime errors during build when DATABASE_URL is not set.
export const db = new Proxy({} as NeonHttpDatabase<typeof schema>, {
  get(_target, prop, receiver) {
    const realDb = getDb();
    const value = Reflect.get(realDb, prop, receiver);
    if (typeof value === 'function') {
      return value.bind(realDb);
    }
    return value;
  },
});

// DB type exported for function signatures. At runtime the actual instance may
// be either a NeonHttpDatabase or NodePgDatabase — the Drizzle query API is
// identical across both adapters.
export type DB = NeonHttpDatabase<typeof schema>;
