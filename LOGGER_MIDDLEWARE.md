# Logger Middleware Documentation

## Overview

The logger middleware has been implemented using [Uber's Zap](https://github.com/uber-go/zap) logger library. It logs all HTTP requests in JSON format with detailed information about each request.

## Implementation

### Files Modified

1. **`internal/platform/handler/middleware.go`**
   - Added `LoggerMiddleware` function that logs HTTP requests
   - Uses Zap's structured logging with JSON output

2. **`cmd/main.go`**
   - Initialized Zap production logger (uses JSON format by default)
   - Applied the logger middleware globally to all routes

3. **`go.mod`**
   - Added `go.uber.org/zap v1.27.1` dependency
   - Added `go.uber.org/multierr v1.10.0` dependency (required by Zap)

## Features

The logger middleware captures and logs:

- **HTTP Method** - The HTTP method used (GET, POST, PUT, DELETE, etc.)
- **URL Path** - The request path/endpoint
- **Status Code** - The HTTP response status code
- **Duration** - Time taken to process the request
- **Success** - Boolean indicating if the request was successful (status code 200-399)

## Log Format

All logs are in JSON format. Example log output:

```json
{
  "level": "info",
  "ts": 1732233600.123456,
  "caller": "handler/middleware.go:115",
  "msg": "HTTP Request",
  "method": "POST",
  "path": "/api/orders",
  "status_code": 201,
  "duration": 0.045,
  "success": true
}
```

### Example Logs

**Successful Request:**
```json
{
  "level": "info",
  "ts": 1732233600.123456,
  "msg": "HTTP Request",
  "method": "GET",
  "path": "/api/products",
  "status_code": 200,
  "duration": 0.012,
  "success": true
}
```

**Failed Request:**
```json
{
  "level": "info",
  "ts": 1732233600.123456,
  "msg": "HTTP Request",
  "method": "POST",
  "path": "/api/orders",
  "status_code": 400,
  "duration": 0.008,
  "success": false
}
```

**Unauthorized Request:**
```json
{
  "level": "info",
  "ts": 1732233600.123456,
  "msg": "HTTP Request",
  "method": "GET",
  "path": "/api/products/123",
  "status_code": 401,
  "duration": 0.003,
  "success": false
}
```

## Architecture Compliance

✅ **Platform Layer Only** - The logger middleware is implemented in the `platform/handler` layer and does not depend on any domain layer implementations.

✅ **No Domain Logic** - The middleware only handles HTTP-specific concerns (request/response logging).

✅ **Dependency Flow** - The middleware is independent and can be used with any HTTP handler without knowledge of domain services.

## Usage

The middleware is automatically applied globally to all routes in `cmd/main.go`:

```go
// Initialize Zap logger (production config uses JSON format)
logger, err := zap.NewProduction()
if err != nil {
    log.Fatalf("Failed to initialize logger: %v", err)
}
defer logger.Sync()

// Apply Logger middleware globally
router.Use(handler.LoggerMiddleware(logger))
```

## Configuration

The logger uses Zap's production configuration which:
- Outputs in JSON format
- Logs at Info level and above
- Includes timestamps and caller information
- Buffers writes for better performance

## Performance

Zap is designed for high-performance logging:
- Zero-allocation JSON encoder
- Structured logging without reflection
- Suitable for high-throughput applications

## Future Enhancements

Potential improvements:
- Add request ID tracking
- Log request/response body for debugging (configurable)
- Add support for different log levels based on status code
- Configure log output destination (file, stdout, external service)

