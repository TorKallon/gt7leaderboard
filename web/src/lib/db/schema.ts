import { pgTable, uuid, text, integer, boolean, real, timestamp, index } from 'drizzle-orm/pg-core';

export const drivers = pgTable('drivers', {
  id: uuid('id').defaultRandom().primaryKey(),
  psnAccountId: text('psn_account_id'),
  psnOnlineId: text('psn_online_id'),
  displayName: text('display_name').notNull(),
  isGuest: boolean('is_guest').default(false).notNull(),
  createdAt: timestamp('created_at', { withTimezone: true }).defaultNow().notNull(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).defaultNow().notNull(),
});

export const tracks = pgTable('tracks', {
  id: uuid('id').defaultRandom().primaryKey(),
  name: text('name').notNull(),
  layout: text('layout').notNull().default(''),
  slug: text('slug').unique().notNull(),
  country: text('country'),
  lengthMeters: integer('length_meters'),
  numCorners: integer('num_corners'),
  hasWeather: boolean('has_weather').default(false).notNull(),
  createdAt: timestamp('created_at', { withTimezone: true }).defaultNow().notNull(),
});

export const cars = pgTable('cars', {
  id: integer('id').primaryKey(), // GT7 internal car ID
  name: text('name').notNull(),
  manufacturer: text('manufacturer').notNull(),
  category: text('category').notNull(), // "Gr.1", "Gr.2", "Gr.3", "Gr.4", "Gr.B", "N", "Gr.X"
  ppStock: real('pp_stock'),
  createdAt: timestamp('created_at', { withTimezone: true }).defaultNow().notNull(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).defaultNow().notNull(),
});

export const sessions = pgTable('sessions', {
  id: uuid('id').defaultRandom().primaryKey(),
  driverId: uuid('driver_id').references(() => drivers.id),
  trackId: uuid('track_id').references(() => tracks.id),
  carId: integer('car_id').references(() => cars.id),
  startedAt: timestamp('started_at', { withTimezone: true }).notNull(),
  endedAt: timestamp('ended_at', { withTimezone: true }),
  detectionMethod: text('detection_method').notNull().default('unmatched'),
  createdAt: timestamp('created_at', { withTimezone: true }).defaultNow().notNull(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).defaultNow().notNull(),
});

export const lapRecords = pgTable('lap_records', {
  id: uuid('id').defaultRandom().primaryKey(),
  driverId: uuid('driver_id').references(() => drivers.id),
  trackId: uuid('track_id').references(() => tracks.id),
  carId: integer('car_id').references(() => cars.id),
  sessionId: uuid('session_id').references(() => sessions.id),
  lapTimeMs: integer('lap_time_ms').notNull(),
  lapNumber: integer('lap_number').notNull(),
  autoDetectedDriverId: uuid('auto_detected_driver_id').references(() => drivers.id),
  weather: text('weather').notNull().default('unknown'),
  isValid: boolean('is_valid').default(true).notNull(),
  recordedAt: timestamp('recorded_at', { withTimezone: true }).notNull(),
  createdAt: timestamp('created_at', { withTimezone: true }).defaultNow().notNull(),
  updatedAt: timestamp('updated_at', { withTimezone: true }).defaultNow().notNull(),
}, (table) => [
  index('idx_lap_records_leaderboard').on(table.trackId, table.carId, table.driverId, table.lapTimeMs),
  index('idx_lap_records_driver').on(table.driverId, table.recordedAt),
  index('idx_lap_records_session').on(table.sessionId, table.lapNumber),
]);

export const collectorHeartbeats = pgTable('collector_heartbeats', {
  id: uuid('id').defaultRandom().primaryKey(),
  status: text('status').notNull(),
  currentSessionId: uuid('current_session_id'),
  uptimeSeconds: integer('uptime_seconds'),
  receivedAt: timestamp('received_at', { withTimezone: true }).defaultNow().notNull(),
});
