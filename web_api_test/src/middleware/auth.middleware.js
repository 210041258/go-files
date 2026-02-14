
const JWT_SECRET = process.env.JWT_SECRET || "supersecret";

export default (req, res, next) => {
  try {
    const authHeader = req.headers.authorization;
    if (!authHeader) {
      return res.status(401).json({ error: "Authorization header missing" });
    }

    const token = authHeader.split(" ")[1];
    if (!token) {
      return res.status(401).json({ error: "Token missing" });
    }

    const decoded = jwt.verify(token, JWT_SECRET);
    req.userId = decoded.id;
    next();
  } catch (err) {
    const statusCode = err.name === "JsonWebTokenError" ? 401 : 500;
    res.status(statusCode).json({ error: "Invalid token" });
  }
  
};
