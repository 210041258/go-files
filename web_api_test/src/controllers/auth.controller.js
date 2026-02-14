
const dbType = process.env.DB_TYPE || "postgres";
const db = require("../db/db.service");

let UserModel;
if (dbType === "mongo") {
  const mongoose = db;
  const userSchema = new mongoose.Schema({
    name: String,
    email: { type: String, unique: true },
    password: String,
  });
  UserModel = mongoose.models.User || mongoose.model("User", userSchema);
}

const JWT_SECRET = process.env.JWT_SECRET || "supersecret";

exports.register = async (req, res) => {
  try {
    const { name, email, password } = req.body;
    const hashed = await bcrypt.hash(password, 10);

    if (dbType === "postgres") {
      await db.query(
        "INSERT INTO users (name, email, password) VALUES ($1, $2, $3)",
        [name, email, hashed]
      );
    } else if (dbType === "mysql") {
      await db.query(
        "INSERT INTO users (name, email, password) VALUES (?, ?, ?)",
        [name, email, hashed]
      );
    } else if (dbType === "mongo") {
      const user = new UserModel({ name, email, password: hashed });
      await user.save();
    }

    res.json({ message: "User registered successfully ✅" });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
};

exports.login = async (req, res) => {
  try {
    const { email, password } = req.body;
    let user;

    if (dbType === "postgres") {
      const result = await db.query("SELECT * FROM users WHERE email=$1", [
        email,
      ]);
      user = result.rows[0];
    } else if (dbType === "mysql") {
      const [rows] = await db.query("SELECT * FROM users WHERE email=?", [
        email,
      ]);
      user = rows[0];
    } else if (dbType === "mongo") {
      user = await UserModel.findOne({ email });
    }

    if (!user) return res.status(400).json({ error: "User not found" });

    const isValid = await bcrypt.compare(password, user.password);
    if (!isValid) return res.status(400).json({ error: "Invalid password" });

    const token = jwt.sign({ id: user.id || user._id }, JWT_SECRET, {
      expiresIn: "1h",
    });

    res.json({ token });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
};
