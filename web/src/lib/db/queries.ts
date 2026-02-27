import { sql } from 'drizzle-orm';
import type { DB } from './index';

/**
 * Returns all tracks with aggregate stats: lap_count, driver_count,
 * record_holder display name, and record_time_ms.
 */
export async function getTrackList(db: DB) {
  const result = await db.execute(sql`
    SELECT
      t.id,
      t.name,
      t.layout,
      t.slug,
      t.country,
      t.length_meters,
      t.num_corners,
      t.has_weather,
      t.created_at,
      COALESCE(stats.lap_count, 0) AS lap_count,
      COALESCE(stats.driver_count, 0) AS driver_count,
      rec.display_name AS record_holder,
      rec.lap_time_ms AS record_time_ms
    FROM tracks t
    LEFT JOIN LATERAL (
      SELECT
        COUNT(*)::int AS lap_count,
        COUNT(DISTINCT lr.driver_id)::int AS driver_count
      FROM lap_records lr
      WHERE lr.track_id = t.id AND lr.is_valid = true
    ) stats ON true
    LEFT JOIN LATERAL (
      SELECT lr.lap_time_ms, d.display_name
      FROM lap_records lr
      JOIN drivers d ON d.id = lr.driver_id
      WHERE lr.track_id = t.id AND lr.is_valid = true
      ORDER BY lr.lap_time_ms ASC
      LIMIT 1
    ) rec ON true
    ORDER BY t.name ASC
  `);
  return result.rows;
}

/**
 * Returns leaderboard for a track: best lap per driver, ranked.
 * Optionally filtered by car category or specific car ID.
 */
export async function getTrackLeaderboard(
  db: DB,
  trackId: string,
  opts?: { category?: string; carId?: number }
) {
  const categoryFilter = opts?.category
    ? sql`AND c.category = ${opts.category}`
    : sql``;
  const carFilter = opts?.carId
    ? sql`AND lr.car_id = ${opts.carId}`
    : sql``;

  const result = await db.execute(sql`
    WITH ranked AS (
      SELECT
        lr.id,
        lr.driver_id,
        lr.lap_time_ms,
        lr.car_id,
        lr.recorded_at,
        ROW_NUMBER() OVER (PARTITION BY lr.driver_id ORDER BY lr.lap_time_ms ASC) AS rn
      FROM lap_records lr
      JOIN cars c ON c.id = lr.car_id
      WHERE lr.track_id = ${trackId}
        AND lr.is_valid = true
        ${categoryFilter}
        ${carFilter}
    )
    SELECT
      ROW_NUMBER() OVER (ORDER BY r.lap_time_ms ASC)::int AS rank,
      r.id AS lap_id,
      r.driver_id,
      d.display_name AS driver_name,
      d.psn_online_id,
      r.lap_time_ms,
      r.car_id,
      c.name AS car_name,
      c.manufacturer AS car_manufacturer,
      c.category AS car_category,
      r.lap_time_ms - FIRST_VALUE(r.lap_time_ms) OVER (ORDER BY r.lap_time_ms ASC) AS delta_to_leader_ms,
      r.recorded_at AS achieved_at
    FROM ranked r
    JOIN drivers d ON d.id = r.driver_id
    JOIN cars c ON c.id = r.car_id
    WHERE r.rn = 1
    ORDER BY r.lap_time_ms ASC
  `);
  return result.rows;
}

/**
 * Returns stats for a driver: total_laps, tracks_driven, favorite track/car.
 */
export async function getDriverStats(db: DB, driverId: string) {
  const result = await db.execute(sql`
    SELECT
      (SELECT COUNT(*)::int FROM lap_records WHERE driver_id = ${driverId}) AS total_laps,
      (SELECT COUNT(DISTINCT track_id)::int FROM lap_records WHERE driver_id = ${driverId}) AS tracks_driven,
      fav_track.name AS favorite_track,
      fav_track.slug AS favorite_track_slug,
      fav_car.name AS favorite_car,
      fav_car.id AS favorite_car_id
    FROM (
      SELECT track_id, COUNT(*) AS cnt
      FROM lap_records
      WHERE driver_id = ${driverId}
      GROUP BY track_id
      ORDER BY cnt DESC
      LIMIT 1
    ) ft
    LEFT JOIN tracks fav_track ON fav_track.id = ft.track_id
    LEFT JOIN LATERAL (
      SELECT car_id, COUNT(*) AS cnt
      FROM lap_records
      WHERE driver_id = ${driverId}
      GROUP BY car_id
      ORDER BY cnt DESC
      LIMIT 1
    ) fc ON true
    LEFT JOIN cars fav_car ON fav_car.id = fc.car_id
  `);

  // If driver has no laps, the above may return empty
  if (result.rows.length === 0) {
    return {
      total_laps: 0,
      tracks_driven: 0,
      favorite_track: null,
      favorite_track_slug: null,
      favorite_car: null,
      favorite_car_id: null,
    };
  }
  return result.rows[0];
}

/**
 * Returns recent laps with driver/track/car info.
 */
export async function getRecentLaps(
  db: DB,
  opts?: { driverId?: string; limit?: number }
) {
  const limit = opts?.limit ?? 50;
  const driverFilter = opts?.driverId
    ? sql`WHERE lr.driver_id = ${opts.driverId}`
    : sql``;

  const result = await db.execute(sql`
    SELECT
      lr.id,
      lr.lap_time_ms,
      lr.lap_number,
      lr.weather,
      lr.is_valid,
      lr.recorded_at,
      lr.session_id,
      d.id AS driver_id,
      d.display_name AS driver_name,
      t.id AS track_id,
      t.name AS track_name,
      t.slug AS track_slug,
      c.id AS car_id,
      c.name AS car_name,
      c.manufacturer AS car_manufacturer
    FROM lap_records lr
    LEFT JOIN drivers d ON d.id = lr.driver_id
    LEFT JOIN tracks t ON t.id = lr.track_id
    LEFT JOIN cars c ON c.id = lr.car_id
    ${driverFilter}
    ORDER BY lr.recorded_at DESC
    LIMIT ${limit}
  `);
  return result.rows;
}

/**
 * Returns track records for specified categories (plus overall).
 * Used to display a records summary on the track detail page.
 */
export async function getTrackRecords(
  db: DB,
  trackId: string,
  categories: string[] = ['Gr.3']
) {
  // Overall record
  const overallResult = await db.execute(sql`
    SELECT lr.lap_time_ms, d.display_name AS driver_name, c.name AS car_name, c.category AS car_category, lr.recorded_at
    FROM lap_records lr
    JOIN drivers d ON d.id = lr.driver_id
    JOIN cars c ON c.id = lr.car_id
    WHERE lr.track_id = ${trackId} AND lr.is_valid = true
    ORDER BY lr.lap_time_ms ASC
    LIMIT 1
  `);

  const records: Array<{
    label: string;
    driver_name: string | null;
    lap_time_ms: number | null;
    car_name: string | null;
    car_category: string | null;
    recorded_at: string | null;
  }> = [{
    label: 'Overall',
    driver_name: overallResult.rows.length > 0 ? overallResult.rows[0].driver_name as string : null,
    lap_time_ms: overallResult.rows.length > 0 ? overallResult.rows[0].lap_time_ms as number : null,
    car_name: overallResult.rows.length > 0 ? overallResult.rows[0].car_name as string : null,
    car_category: overallResult.rows.length > 0 ? overallResult.rows[0].car_category as string : null,
    recorded_at: overallResult.rows.length > 0 ? overallResult.rows[0].recorded_at as string : null,
  }];

  // Category records
  for (const cat of categories) {
    const catResult = await db.execute(sql`
      SELECT lr.lap_time_ms, d.display_name AS driver_name, c.name AS car_name, c.category AS car_category, lr.recorded_at
      FROM lap_records lr
      JOIN drivers d ON d.id = lr.driver_id
      JOIN cars c ON c.id = lr.car_id
      WHERE lr.track_id = ${trackId} AND lr.is_valid = true AND c.category = ${cat}
      ORDER BY lr.lap_time_ms ASC
      LIMIT 1
    `);

    records.push({
      label: cat,
      driver_name: catResult.rows.length > 0 ? catResult.rows[0].driver_name as string : null,
      lap_time_ms: catResult.rows.length > 0 ? catResult.rows[0].lap_time_ms as number : null,
      car_name: catResult.rows.length > 0 ? catResult.rows[0].car_name as string : null,
      car_category: catResult.rows.length > 0 ? catResult.rows[0].car_category as string : null,
      recorded_at: catResult.rows.length > 0 ? catResult.rows[0].recorded_at as string : null,
    });
  }

  return records;
}

/**
 * Check if a lap is an overall, category, or car record for the given track.
 * Returns an array of record types broken.
 */
export async function checkForRecords(
  db: DB,
  trackId: string,
  carId: number,
  lapTimeMs: number
) {
  const records: Array<{
    type: 'overall' | 'category' | 'car';
    previousTimeMs: number | null;
    previousDriver: string | null;
  }> = [];

  // Check overall track record
  const overallResult = await db.execute(sql`
    SELECT lr.lap_time_ms, d.display_name
    FROM lap_records lr
    LEFT JOIN drivers d ON d.id = lr.driver_id
    WHERE lr.track_id = ${trackId} AND lr.is_valid = true
    ORDER BY lr.lap_time_ms ASC
    LIMIT 1
  `);

  if (
    overallResult.rows.length === 0 ||
    lapTimeMs < (overallResult.rows[0].lap_time_ms as number)
  ) {
    records.push({
      type: 'overall',
      previousTimeMs: overallResult.rows.length > 0
        ? (overallResult.rows[0].lap_time_ms as number)
        : null,
      previousDriver: overallResult.rows.length > 0
        ? (overallResult.rows[0].display_name as string)
        : null,
    });
  }

  // Check category record
  const categoryResult = await db.execute(sql`
    SELECT lr.lap_time_ms, d.display_name
    FROM lap_records lr
    LEFT JOIN drivers d ON d.id = lr.driver_id
    JOIN cars c ON c.id = lr.car_id
    WHERE lr.track_id = ${trackId}
      AND lr.is_valid = true
      AND c.category = (SELECT category FROM cars WHERE id = ${carId})
    ORDER BY lr.lap_time_ms ASC
    LIMIT 1
  `);

  if (
    categoryResult.rows.length === 0 ||
    lapTimeMs < (categoryResult.rows[0].lap_time_ms as number)
  ) {
    records.push({
      type: 'category',
      previousTimeMs: categoryResult.rows.length > 0
        ? (categoryResult.rows[0].lap_time_ms as number)
        : null,
      previousDriver: categoryResult.rows.length > 0
        ? (categoryResult.rows[0].display_name as string)
        : null,
    });
  }

  // Check car-specific record
  const carResult = await db.execute(sql`
    SELECT lr.lap_time_ms, d.display_name
    FROM lap_records lr
    LEFT JOIN drivers d ON d.id = lr.driver_id
    WHERE lr.track_id = ${trackId}
      AND lr.car_id = ${carId}
      AND lr.is_valid = true
    ORDER BY lr.lap_time_ms ASC
    LIMIT 1
  `);

  if (
    carResult.rows.length === 0 ||
    lapTimeMs < (carResult.rows[0].lap_time_ms as number)
  ) {
    records.push({
      type: 'car',
      previousTimeMs: carResult.rows.length > 0
        ? (carResult.rows[0].lap_time_ms as number)
        : null,
      previousDriver: carResult.rows.length > 0
        ? (carResult.rows[0].display_name as string)
        : null,
    });
  }

  return records;
}
