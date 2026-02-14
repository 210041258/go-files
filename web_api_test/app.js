

// Import routes
import userRoutes from "./routes/user.routes.js";
import uploadRoutes from "./routes/upload.routes.js";
import authRoutes from "./routes/auth.routes.js";

// Import error handling middleware
import { errorHandler, notFound } from "./middleware/error.middleware.js";

// Get __dirname equivalent in ES modules
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const app = express();

// ====================================
// Security & Production Middleware
// ====================================


// Rate limiting
const limiter = rateLimit({
    windowMs: 15 * 60 * 1000, // 15 minutes
    max: 100, // Limit each IP to 100 requests per windowMs
    message: {
        error: "Too many requests from this IP, please try again later.",
    },
    standardHeaders: true,
    legacyHeaders: false,
});

// Apply rate limiting to all routes
app.use("/api/", limiter);

// ====================================
// Basic Middleware
// ====================================

// CORS configuration
const corsOptions = {
    origin: process.env.CORS_ORIGIN || "*",
    credentials: true,
    optionsSuccessStatus: 200,
    methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"],
    allowedHeaders: [
        "Content-Type",
        "Authorization",
        "X-Requested-With",
        "Accept",
        "Origin",
    ],
};

app.use(cors(corsOptions));
app.options("*", cors(corsOptions)); // Pre-flight requests

// Body parsers
app.use(express.json({ limit: "10mb" }));
app.use(express.urlencoded({ extended: true, limit: "10mb" }));
app.use(cookieParser());

// Security middlewares
app.use(mongoSanitize()); // Prevent NoSQL injection
app.use(hpp()); // Prevent HTTP Parameter Pollution

// Compression (for production)
if (process.env.NODE_ENV === "production") {
    app.use(compression());
}

// Request logging
const morganFormat = process.env.NODE_ENV === "production" ? "combined" : "dev";
app.use(morgan(morganFormat));

// ====================================
// Static Files
// ====================================

// Serve uploads directory with security headers
app.use(
    "/uploads",
    express.static(path.join(__dirname, "../uploads"), {
        setHeaders: (res, filePath) => {
            // Add security headers for static files
            res.setHeader("X-Content-Type-Options", "nosniff");
            
            // Cache control for uploaded files (1 hour)
            res.setHeader("Cache-Control", "public, max-age=3600");
        },
    })
);

// ====================================
// Routes (API Versioning)
// ====================================

// API routes with version prefix
app.use("/api/v1/users", userRoutes);
app.use("/api/v1/upload", uploadRoutes);
app.use("/api/v1/auth", authRoutes);

// ====================================
// Health & Status Endpoints
// ====================================

// Health check (for load balancers & k8s)
app.get("/health", (req, res) => {
    const healthcheck = {
        status: "healthy",
        timestamp: new Date().toISOString(),
        uptime: process.uptime(),
        memory: process.memoryUsage(),
        pid: process.pid,
        nodeVersion: process.version,
        platform: process.platform,
    };
    
    res.status(200).json(healthcheck);
});

// Readiness probe (for k8s)
app.get("/ready", (req, res) => {
    // Add database connection checks here if needed
    const readiness = {
        ready: true,
        timestamp: new Date().toISOString(),
        checks: {
            database: true, // You can implement actual DB checks
        },
    };
    
    res.status(200).json(readiness);
});

// Liveness probe
app.get("/live", (req, res) => {
    res.status(200).json({ live: true });
});

// API documentation/info endpoint
app.get("/", (req, res) => {
    const apiInfo = {
        message: "API Server is running",
        version: "1.0.0",
        environment: process.env.NODE_ENV || "development",
        timestamp: new Date().toISOString(),
        documentation: process.env.API_DOCS_URL || null,
        endpoints: {
            auth: {
                login: "POST /api/v1/auth/login",
                register: "POST /api/v1/auth/register",
                refresh: "POST /api/v1/auth/refresh",
                logout: "POST /api/v1/auth/logout",
            },
            users: {
                list: "GET /api/v1/users",
                get: "GET /api/v1/users/:id",
                update: "PUT /api/v1/users/:id",
                delete: "DELETE /api/v1/users/:id",
            },
            upload: {
                single: "POST /api/v1/upload",
                multiple: "POST /api/v1/upload/multiple",
            },
            status: {
                health: "GET /health",
                ready: "GET /ready",
                live: "GET /live",
            },
        },
    };
    
    res.json(apiInfo);
});

// ====================================
// Error Handling
// ====================================

// 404 handler
app.use(notFound);

// Global error handler
app.use(errorHandler);

export default app;