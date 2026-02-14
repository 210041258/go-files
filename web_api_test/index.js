import app from "./app.js";

// Load environment variables first
dotenv.config();

const PORT = process.env.APP_PORT || 3000;
const HOST = process.env.HOST || "0.0.0.0"; // Changed to 0.0.0.0 for Docker compatibility
const NODE_ENV = process.env.NODE_ENV || "development";

// Validate required environment variables
const requiredEnvVars = ["JWT_SECRET"];
const missingEnvVars = requiredEnvVars.filter(
  (varName) => !process.env[varName]
);

if (missingEnvVars.length > 0) {
  console.error("❌ Missing required environment variables:");
  missingEnvVars.forEach((varName) => {
    console.error(`   - ${varName}`);
  });
  console.error("🚨 Server cannot start without these variables");
  process.exit(1);
}

// Function to display startup banner
const displayStartupBanner = () => {
  console.log(`
┌─────────────────────────────────────────────────────┐
│                                                     │
│                   🚀 API SERVER                     │
│                                                     │
├─────────────────────────────────────────────────────┤
│                                                     │
│  📍 Environment: ${NODE_ENV.padEnd(30)}│
│  🌐 Host: ${HOST.padEnd(34)}│
│  🔌 Port: ${PORT.toString().padEnd(34)}│
│  ⏰ Time: ${new Date().toISOString().padEnd(24)}│
│                                                     │
│  🔗 Local: http://localhost:${PORT.toString().padEnd(18)}│
│  🌍 Network: http://${HOST}:${PORT.toString().padEnd(18)}│
│                                                     │
├─────────────────────────────────────────────────────┤
│                                                     │
│  📊 Health: http://${HOST}:${PORT}/health         │
│  📈 Ready: http://${HOST}:${PORT}/ready          │
│  📋 Live: http://${HOST}:${PORT}/live            │
│                                                     │
└─────────────────────────────────────────────────────┘
`);
};

// Function to display system information
const displaySystemInfo = () => {
  console.log("\n📊 System Information:");
  console.log(`   Node.js: ${process.version}`);
  console.log(`   Platform: ${process.platform}/${process.arch}`);
  console.log(`   PID: ${process.pid}`);
  console.log(`   Uptime: ${process.uptime().toFixed(2)} seconds`);
  console.log(
    `   Memory: ${(process.memoryUsage().heapUsed / 1024 / 1024).toFixed(2)} MB`
  );
};

let isShuttingDown = false;

// Graceful shutdown function
const gracefulShutdown = async (signal) => {
  if (isShuttingDown) return;
  isShuttingDown = true;

  console.log(`\n🛑 Received ${signal}. Starting graceful shutdown...`);
  console.log("⏳ Closing HTTP server...");

  // Set a timeout to force shutdown if graceful shutdown takes too long
  const forceShutdownTimeout = setTimeout(() => {
    console.error("⚠️ Graceful shutdown timed out. Forcing exit...");
    process.exit(1);
  }, 10000);

  try {
    // Close the server
    server.close(() => {
      clearTimeout(forceShutdownTimeout);
      console.log("✅ HTTP server closed");

      // Here you can add cleanup for other resources
      // e.g., database connections, file handles, etc.
      console.log("✨ Cleanup completed");
      console.log("👋 Goodbye!");
      process.exit(0);
    });

    // Close any existing connections
    server.closeIdleConnections();

    // If there are still connections after 5 seconds, close them
    setTimeout(() => {
      server.closeAllConnections();
    }, 5000);
  } catch (error) {
    console.error("❌ Error during shutdown:", error);
    process.exit(1);
  }
};

// Start the server
const server = app.listen(PORT, HOST, (error) => {
  if (error) {
    console.error("❌ Failed to start server:", error.message);

    if (error.code === "EADDRINUSE") {
      console.error(`   Port ${PORT} is already in use`);
      console.log("💡 Try one of these solutions:");
      console.log("   1. Use a different port by setting APP_PORT environment variable");
      console.log("   2. Find and kill the process using port", PORT);
      console.log(`      Command: lsof -ti:${PORT} | xargs kill -9`);
    } else if (error.code === "EACCES") {
      console.error(`   Permission denied for port ${PORT}`);
      console.log("💡 Try using a port above 1024 or run with sudo");
    }

    process.exit(1);
  }

  displayStartupBanner();
  displaySystemInfo();
});

// Server error handler
server.on("error", (error) => {
  console.error("🔥 Server error occurred:", error.message);
  
  // Only exit if it's a critical error
  if (error.code === "EADDRINUSE" || error.code === "EACCES") {
    process.exit(1);
  }
});

// Handle different shutdown signals
const shutdownSignals = ["SIGTERM", "SIGINT", "SIGUSR2"];
shutdownSignals.forEach((signal) => {
  process.on(signal, () => gracefulShutdown(signal));
});

// Handle unhandled rejections (promises)
process.on("unhandledRejection", (reason, promise) => {
  console.error("🚨 Unhandled Promise Rejection:");
  console.error("   Reason:", reason);
  console.error("   Promise:", promise);
  
  // In production, you might want to log to a monitoring service
  if (NODE_ENV === "production") {
    // Log to monitoring service (e.g., Sentry, Datadog)
    console.error("📝 Logging to monitoring service...");
  }
});

// Handle uncaught exceptions
process.on("uncaughtException", (error) => {
  console.error("💥 Uncaught Exception:", error.message);
  console.error(error.stack);
  
  // Attempt graceful shutdown on uncaught exception
  gracefulShutdown("UNCAUGHT_EXCEPTION");
});

// Handle process warnings
process.on("warning", (warning) => {
  console.warn("⚠️ Process Warning:");
  console.warn("   Name:", warning.name);
  console.warn("   Message:", warning.message);
  console.warn("   Stack:", warning.stack);
});

// Export the server for testing purposes
export { server };