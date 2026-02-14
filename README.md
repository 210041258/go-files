# Go TestUtils

![Go Version](https://img.shields.io/badge/Go-1.22%2B-blue)
![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20Windows%20%7C%20macOS-green)
![Status](https://img.shields.io/badge/Status-Production%20Ready-brightgreen)

Enterprise-grade utility framework for building, testing, and operating
high-performance web APIs and distributed systems in Go.


------------------------------------------------------------------------

# 📑 Table of Contents

-   [Overview](#overview)
-   [Why Go TestUtils](#why-go-testutils)
-   [Installation](#installation)
-   [Architecture](#architecture)
-   [Core Modules](#core-modules)
    -   [Networking & Communication](#networking--communication)
    -   [Cryptography & Security](#cryptography--security)
    -   [Concurrency & Async](#concurrency--async)
    -   [Storage & Databases](#storage--databases)
    -   [System & OS Utilities](#system--os-utilities)
    -   [Business & Payment Simulation](#business--payment-simulation)
-   [Example Usage](#example-usage)
-   [Platform Support](#platform-support)
-   [Performance Principles](#performance-principles)
-   [Security Philosophy](#security-philosophy)
-   [Contributing](#contributing)
-   [Roadmap](#roadmap)
-   [License](#license)

------------------------------------------------------------------------

# Overview

Go TestUtils is a modular backend toolkit designed for:

-   Microservices (HTTP, gRPC, Gateway simulation)
-   Fintech & payment systems
-   High-concurrency architectures
-   Infrastructure & DevOps tooling
-   Performance testing & diagnostics

Cross-platform support included (Linux, Windows, macOS).

------------------------------------------------------------------------

# Why Go TestUtils

✔ Modular architecture\
✔ Concurrency optimized\
✔ Cross-platform ready\
✔ Payment & gateway simulation\
✔ Production-grade retry & rate limiting\
✔ Testing & mocking utilities

------------------------------------------------------------------------

# Installation

``` bash
go get github.com/your-org/testutils
```

Or local setup:

``` bash
git clone https://github.com/your-org/testutils.git
cd testutils
go mod tidy
```

------------------------------------------------------------------------

# Architecture

    Business Layer
        ↓
    Concurrency Layer
        ↓
    Networking Layer
        ↓
    System & Storage Layer

Layered structure ensures separation of concerns and scalability.

------------------------------------------------------------------------

# Core Modules

## Networking & Communication

-   HTTP / TCP / WebSocket servers
-   gRPC utilities
-   Service discovery
-   DNS & IP tools
-   Gateway bridging
-   Request/Response builders
-   Port scanning & validation

## Cryptography & Security

-   AES (CBC, CTR, GCM)
-   JWT & token handling
-   SHA / MD5 hashing
-   Secure key management
-   Encoding / decoding utilities

## Concurrency & Async

-   Worker pools
-   Lifecycle managers
-   Rate limiting (Token / Leaky bucket)
-   Retry with exponential backoff
-   Timeout handling
-   Graceful shutdown

## Storage & Databases

-   PostgreSQL store
-   Redis store
-   SQLite store
-   Generic repository interfaces
-   Deduplication logic

## System & OS Utilities

-   CPU & memory tracking
-   Disk statistics
-   File abstraction
-   Process & signal handling
-   Runtime introspection

## Business & Payment Simulation

-   Payment client/server
-   Gateway jitter & timeout simulation
-   Dead-letter queues
-   Feature flags
-   Test data management
-   Distributed tracing

------------------------------------------------------------------------

# Example Usage

## Start HTTP Server

``` go
srv := testutils.NewHTTPServer(":8080")
srv.Start()
defer srv.Stop()
```

## Rate Limiting

``` go
limiter := testutils.NewTokenBucket(100, time.Second)
if limiter.Allow() {
    processRequest()
}
```

## AES Encryption

``` go
ciphertext, err := testutils.EncryptAES(key, plaintext)
```

------------------------------------------------------------------------

# Platform Support

  OS        Supported
  --------- -----------
  Linux     ✅
  Windows   ✅
  macOS     ✅

------------------------------------------------------------------------

# Performance Principles

-   Minimal allocations
-   Lock-efficient concurrency
-   Context-aware execution
-   High-throughput networking
-   Optimized retry algorithms

------------------------------------------------------------------------

# Security Philosophy

-   Secure defaults
-   Authenticated encryption
-   Explicit key handling
-   Safe secret management
-   Replay & timeout simulation for fintech systems

------------------------------------------------------------------------

# Contributing

1.  Fork repository
2.  Create feature branch
3.  Add tests
4.  Run `go test ./...`
5.  Submit PR

------------------------------------------------------------------------

# Roadmap

-   Prometheus exporter
-   OpenTelemetry integration
-   Circuit breaker implementation
-   CLI toolkit
-   Observability dashboard support

------------------------------------------------------------------------

# License

Internal / Custom

------------------------------------------------------------------------

**Go TestUtils --- Enterprise Backend Toolkit for Go Systems**
