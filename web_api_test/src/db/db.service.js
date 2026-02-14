// src/db/db.service.js

/*import { pool } from './postgres.js';*/
import { Pool } from 'pg';

// Create a new pool instance with your database configuration
const pool = new Pool
({
  user
: 'postgres',
  host: 'localhost',
  database: 'database',
  password: 'Ahmed@2026',
  port: 5432,
}); 



export async function queryDatabase(queryText, params) {
  try {
    const result = await pool.query(queryText, params);
    return result.rows;
  } catch (err) {
    console.error('Database query error:', err);
    throw err;
  }
}


