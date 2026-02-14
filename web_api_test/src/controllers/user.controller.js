// src/controllers/user.controller.js
const db = require('../db/db.service');

async function getUsers(req, res) {
  try {
    const result = await db.query('SELECT * FROM users');
    res.json(result.rows);
  } catch (err) {
    res.status(500).json({ error: 'Database error' });
  }
}

module.exports = { getUsers };
