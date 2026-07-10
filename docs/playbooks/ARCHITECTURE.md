# Backend Architecture Reference

> Go backend using Clean Architecture + DDD + Hexagonal (Ports & Adapters) + Event-Driven patterns.
> Use this document as a blueprint for new backend projects.

---

## Table of Contents

- [Project Structure](#project-structure)
- [Dependency Injection](#dependency-injection)
- [Domain Layer](#domain-layer)
  - [Aggregates](#aggregates)
  - [Value Objects](#value-objects)
  - [Domain Errors](#domain-errors)
  - [DTOs](#dtos)
  - [Commands](#commands)
- [Application Layer (Services)](#application-layer-services)
- [Ports (Interfaces)](#ports-interfaces)
- [Infrastructure Layer (Adapters)](#infrastructure-layer-adapters)
  - [Repositories (PostgreSQL + GORM)](#repositories-postgresql--gorm)
  - [Unit of Work (Transactions)](#unit-of-work-transactions)
  - [External Services](#external-services)
  - [Event Bus](#event-bus)
  - [SSE (Server-Sent Events)](#sse-server-sent-events)
  - [Cron Jobs](#cron-jobs)
- [API Layer](#api-layer)
  - [HTTP Handlers](#http-handlers)
  - [Middleware Stack](#middleware-stack)
  - [Route Organization](#route-organization)
- [Permission System](#permission-system)
- [Error Handling](#error-handling)
- [Configuration](#configuration)
- [Key Conventions](#key-conventions)

---

## Project Structure

```
project-root/
├── cmd/
│   └── main.go                        # Entry point — all DI wiring happens here
├── internal/
│   ├── domain/                        # Core business logic (no external dependencies)
│   │   ├── aggregate/                 # DDD aggregates (one dir per aggregate)
│   │   │   ├── product/
│   │   │   │   ├── product.go         # Aggregate root
│   │   │   │   ├── sku.go             # Value object
│   │   │   │   ├── product_type.go    # Value object (enum)
│   │   │   │   ├── unit_of_measure.go # Value object (enum)
│   │   │   │   └── error/             # Aggregate-specific errors
│   │   │   ├── supplier/
│   │   │   ├── user/
│   │   │   ├── open_bill/             # Order aggregate
│   │   │   ├── bill/
│   │   │   ├── expense/
│   │   │   └── purchase_entry/
│   │   ├── dto/                       # Data Transfer Objects (domain ↔ service ↔ handler)
│   │   ├── error/                     # Domain-wide error definitions
│   │   ├── ports/                     # Interfaces (contracts for adapters)
│   │   ├── service/                   # Use cases / application services
│   │   ├── command/                   # Command objects (imperative operations)
│   │   └── permissions/               # Role-based permission constants
│   └── platform/                      # Infrastructure / adapters
│       ├── config/                    # Environment-based configuration
│       ├── handler/                   # HTTP handlers + middleware (Gin)
│       ├── postgres/
│       │   ├── repository/            # GORM repository implementations
│       │   ├── migrations/            # SQL migration files
│       │   └── unit_of_work.go        # Transaction wrapper
│       ├── sse/                       # Server-Sent Events hub
│       ├── storage/                   # External storage (S3-compatible)
│       ├── httpclient/                # HTTP clients for external APIs
│       ├── eventbus/                  # Event bus wiring (platform-specific)
│       ├── cron/                      # Scheduled tasks
│       ├── dto/                       # API-level request/response DTOs
│       └── shared/
│           ├── domain/
│           │   ├── error/             # BaseError with stack traces
│           │   └── event/             # BaseEvent for domain events
│           ├── constants/
│           └── utils/
├── pkg/                               # Public, reusable packages
│   ├── eventbus/                      # Watermill-based event bus implementation
│   │   ├── publisher.go               # GoChannelEventBus
│   │   ├── subscriber.go             # GoChannelEventSubscriber
│   │   ├── handler.go                # TypedEventHandler[T] (generic)
│   │   ├── product_lock_manager.go   # Concurrency control
│   │   └── logger.go                 # Zap adapter for Watermill
│   └── domain/
│       └── ports/
│           └── event_bus.go           # Event, EventHandler, EventBus, EventSubscriber interfaces
├── pem/                               # TLS certificates
└── scripts/                           # Database seeds & utilities
```

### Layer Dependencies (Inward Only)

```
Handler (API) → Service (Application) → Ports (Interfaces) ← Adapters (Infrastructure)
                                         ↑
                                    Aggregates (Domain)
```

- **Domain** depends on nothing external.
- **Services** depend on domain + ports (interfaces only).
- **Adapters** implement ports and depend on external libraries (GORM, S3, etc.).
- **Handlers** depend on services and platform DTOs.

---

## Dependency Injection

All wiring is done manually in `cmd/main.go`. No DI framework is used.

```go
func main() {
    // 1. Load configuration
    cfg, _ := config.NewConfig()

    // 2. Initialize database
    db, _ := postgres.NewDB(cfg)

    // 3. Create repositories (implement ports)
    productRepo := repository.NewProductRepository(db)
    orderRepo   := repository.NewOpenBillRepository(db)
    stockRepo   := repository.NewStockRepository(db)
    // ...more repositories

    // 4. Create infrastructure services
    unitOfWork    := postgres.NewUnitOfWork(db)
    storageClient := storage.NewS3Client(cfg)
    jwtService    := service.NewJWTService(cfg.JWTSecret)

    // 5. Create event bus
    eventBus := eventbus.NewGoChannelEventBus(watermillLogger)

    // 6. Create SSE hubs
    sseHub := sse.NewHub()

    // 7. Create application services (inject ports)
    orderService := service.NewOrderService(orderRepo, productRepo, eventBus, unitOfWork, sseHub)
    productService := service.NewProductService(productRepo)
    // ...more services

    // 8. Create event subscribers & register handlers
    subscriber, _ := eventbus.NewGoChannelEventSubscriber(eventBus.PubSub(), watermillLogger)
    subscriber.Subscribe(eventbus.NewTypedEventHandler(dto.OrderCreatedEventName, stockHandler.HandleOrderCreated))
    subscriber.Subscribe(eventbus.NewTypedEventHandler(dto.OrderCreatedEventName, orderService.HandleOrderCreatedSSE))
    go subscriber.Start(ctx)

    // 9. Create HTTP handlers
    orderHandler := handler.NewOrderHandler(orderService)

    // 10. Setup router with middleware and routes
    router := gin.Default()
    // ...register routes with middleware
}
```

**Key principle**: Dependencies flow inward. Concrete implementations are created in `main.go` and injected as interfaces into services.

---

## Domain Layer

### Aggregates

Each aggregate is a self-contained directory under `internal/domain/aggregate/`. An aggregate encapsulates:
- **Private fields** (no exported struct fields — access via methods only)
- **Factory methods** for creation with validation
- **Conversion methods** to/from DTOs
- **Business logic** (calculations, state transitions)
- **Value objects** as separate files in the same package
- **Aggregate-specific errors** in an `error/` subdirectory

**Pattern**:

```go
// internal/domain/aggregate/product/product.go

type Aggregate struct {
    id            string
    name          string
    sku           *SKU              // Value object
    productType   ProductType       // Value object (enum)
    unitPrice     decimal.Decimal   // Shopspring decimal for precision
    vat           decimal.Decimal
    createdAt     time.Time
    updatedAt     time.Time
}

// Factory: Create from request DTO (validates and builds)
func NewAggregateFromCreateProductRequest(req *dto.CreateProductRequest) (*Aggregate, error) {
    sku, err := NewSKU(req.SKU)
    if err != nil {
        return nil, err
    }
    // ...validate, calculate derived fields
    return &Aggregate{id: uuid.Must(uuid.NewV7()).String(), sku: sku, ...}, nil
}

// Factory: Reconstitute from persistence DTO
func NewAggregateFromDTO(d *dto.Product) (*Aggregate, error) {
    // ...rebuild from stored data
}

// Convert to DTO for output
func (a *Aggregate) ToDTO() *dto.Product {
    return &dto.Product{ID: a.id, Name: a.name, ...}
}

// Business logic
func (a *Aggregate) Update(req *dto.UpdateProductRequest) (*Aggregate, error) {
    // ...validate and apply changes
}
```

**Conventions**:
- Aggregate struct fields are always **unexported** (lowercase).
- All creation goes through factory methods — never construct directly.
- Business rules (tax calculations, validation) live inside the aggregate.
- Use `decimal.Decimal` (shopspring) for monetary values — never `float64`.
- UUIDs (v7) as primary keys (`uuid.Must(uuid.NewV7()).String()`).

### Value Objects

Immutable, self-validating types that represent domain concepts.

```go
// internal/domain/aggregate/product/sku.go

type SKU struct {
    value string
}

func NewSKU(value string) (*SKU, error) {
    if !isAlphanumeric(value) {
        return nil, productError.NewInvalidSKUError(value)
    }
    return &SKU{value: value}, nil
}

func (s *SKU) Value() string { return s.value }
```

**Common value objects**: SKU, Email, Phone, Name, ProductType (enum), UnitOfMeasure (enum), PaymentCode (enum).

### Domain Errors

Each aggregate has its own `error/` subdirectory with typed errors.

```go
// internal/domain/aggregate/product/error/product_error.go

type ErrorCode string

const (
    PRODUCT_INVALID_SKU  ErrorCode = "PRODUCT_INVALID_SKU"
    PRODUCT_MISSING_NAME ErrorCode = "PRODUCT_MISSING_NAME"
)

func NewInvalidSKUError(value string) *baseError.BaseError {
    return baseError.NewBaseError(string(PRODUCT_INVALID_SKU), "Invalid SKU format", value)
}
```

### DTOs

Located in `internal/domain/dto/`. Used for communication between layers.

```go
// Entity/response DTOs
type Product struct {
    ID            string          `json:"id"`
    Name          string          `json:"name"`
    SKU           string          `json:"sku"`
    UnitPrice     decimal.Decimal `json:"unit_price"`
    // ...
}

// Request DTOs
type CreateProductRequest struct {
    Name          string          `json:"name" binding:"required"`
    SKU           string          `json:"sku" binding:"required"`
    // ...
}

// Event DTOs (implement ports.Event)
type OrderCreatedEvent struct {
    event.BaseEvent
    OpenBillID string                     `json:"open_bill_id"`
    Products   []OrderCreatedEventProduct `json:"products"`
}

func (e OrderCreatedEvent) EventName() string { return OrderCreatedEventName }
func (e OrderCreatedEvent) Data() []byte      { data, _ := json.Marshal(e); return data }
```

**Convention**: Event DTOs embed `BaseEvent` and implement `EventName()` + `Data()`.

### Commands

Command objects for complex imperative operations.

```go
// internal/domain/command/pay_order_command.go

type PayOrderCommand struct {
    OrderID     string
    PaymentCode string
    PaidByID    string
}
```

---

## Application Layer (Services)

Services are thin orchestrators. They:
1. Accept request DTOs
2. Create/reconstitute aggregates
3. Execute business logic on aggregates
4. Persist via repository ports
5. Publish domain events
6. Return response DTOs

```go
// internal/domain/service/product_service.go

type ProductService struct {
    productRepo ports.ProductRepository  // Injected interface
}

func NewProductService(productRepo ports.ProductRepository) *ProductService {
    return &ProductService{productRepo: productRepo}
}

func (s *ProductService) CreateProduct(ctx context.Context, req *dto.CreateProductRequest) (*dto.Product, error) {
    // 1. Build aggregate (validation happens here)
    aggregate, err := product.NewAggregateFromCreateProductRequest(req)
    if err != nil {
        return nil, err
    }

    // 2. Persist via port
    if err := s.productRepo.Create(ctx, aggregate); err != nil {
        return nil, err
    }

    // 3. Return DTO
    return aggregate.ToDTO(), nil
}
```

**Services with events**:

```go
type OrderService struct {
    orderRepo   ports.OpenBillRepository
    productRepo ports.ProductRepository
    eventBus    ports.EventBus
    unitOfWork  ports.UnitOfWork
    sseNotifier ports.SSENotifier
}

func (s *OrderService) CreateOrder(ctx context.Context, req *dto.CreateOrderRequest, user dto.UserDomain) (*dto.OpenBill, error) {
    var result *dto.OpenBill

    // Wrap in transaction
    err := s.unitOfWork.Do(ctx, func(txCtx context.Context) error {
        // ...business logic with txCtx for transactional consistency
        result = openBill

        // Publish event (after successful persistence)
        return s.eventBus.Publish(txCtx, dto.NewOrderCreatedEvent(...))
    })

    return result, err
}
```

**Event handler methods** also live on services:

```go
func (s *OrderService) HandleOrderCreatedSSE(ctx context.Context, event dto.OrderCreatedEvent) error {
    // Send SSE notification to connected clients
    s.sseNotifier.NotifyArea(ctx, event.Area, "order_created", event)
    return nil
}
```

---

## Ports (Interfaces)

All interface contracts live in `internal/domain/ports/` and `pkg/domain/ports/`.

**Repository ports**:

```go
type ProductRepository interface {
    Create(ctx context.Context, product *product.Aggregate) error
    Update(ctx context.Context, id string, product *product.Aggregate) error
    Delete(ctx context.Context, id string) error
    FindAll(ctx context.Context) ([]*dto.Product, error)
    FindByID(ctx context.Context, id string) (*dto.Product, error)
    FindByIDs(ctx context.Context, ids []string) ([]*dto.Product, error)
}
```

**Event ports** (`pkg/domain/ports/event_bus.go`):

```go
type Event interface {
    EventName() string
}

type EventHandler interface {
    Handle(ctx context.Context, payload []byte) error
    EventName() string
}

type EventBus interface {
    Publish(ctx context.Context, event Event) error
}

type EventSubscriber interface {
    Subscribe(handler EventHandler) error
    Start(ctx context.Context) error
    Close() error
}
```

**Infrastructure ports**:

```go
type UnitOfWork interface {
    Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type SSENotifier interface {
    NotifyArea(ctx context.Context, area string, eventType string, data interface{})
}

type StorageClient interface {
    Upload(ctx context.Context, key string, data []byte, contentType string) error
    Download(ctx context.Context, key string) ([]byte, error)
    Delete(ctx context.Context, key string) error
    GetPublicURL(key string) string
}
```

**Convention**: Ports accept and return domain types (aggregates, DTOs) — never GORM models or infrastructure types.

---

## Infrastructure Layer (Adapters)

### Repositories (PostgreSQL + GORM)

Located in `internal/platform/postgres/repository/`.

Each repository has:
- An **internal GORM model** (unexported, with `gorm` tags)
- **Mapping functions** between GORM models and domain aggregates/DTOs
- **Implementation of the port interface**

```go
// internal/platform/postgres/repository/product_repository.go

// Internal GORM model (never exported)
type productModel struct {
    ID            string          `gorm:"type:uuid;primaryKey"`
    Name          string          `gorm:"type:varchar(255)"`
    UnitPrice     decimal.Decimal `gorm:"type:numeric(19,4)"`
    DeletedAt     *time.Time      // Soft delete
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

func (productModel) TableName() string { return "products" }

type ProductRepository struct {
    db *gorm.DB
}

func NewProductRepository(db *gorm.DB) ports.ProductRepository {
    return &ProductRepository{db: db}
}

func (r *ProductRepository) Create(ctx context.Context, product *product.Aggregate) error {
    model := toProductModel(product)  // Aggregate → GORM model
    return r.getDB(ctx).Create(&model).Error
}

func (r *ProductRepository) FindByID(ctx context.Context, id string) (*dto.Product, error) {
    var model productModel
    if err := r.getDB(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&model).Error; err != nil {
        return nil, err
    }
    return toProductDTO(&model), nil  // GORM model → DTO
}

// Transaction-aware DB getter
func (r *ProductRepository) getDB(ctx context.Context) *gorm.DB {
    if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
        return tx  // Use transaction if available
    }
    return r.db
}
```

**Conventions**:
- Soft deletes via `deleted_at IS NULL` in queries.
- `numeric(19,4)` for decimal precision.
- PostgreSQL `gen_random_uuid()` for UUID generation.
- Transaction-aware via context (checks for `txKey{}`).
- Return `ports.XxxRepository` from constructor (not concrete type).

### Unit of Work (Transactions)

```go
// internal/platform/postgres/unit_of_work.go

type txKey struct{}

type UnitOfWork struct {
    db *gorm.DB
}

func (u *UnitOfWork) Do(ctx context.Context, fn func(ctx context.Context) error) error {
    return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        txCtx := context.WithValue(ctx, txKey{}, tx)
        return fn(txCtx)
    })
}
```

Repositories check the context for a transaction via `txKey{}`. This allows multiple repositories to participate in the same transaction transparently.

### External Services

**AWS S3 storage** (`internal/platform/storage/s3_client.go`):

```go
type S3Client struct {
    client    *s3.Client
    bucket    string
    region    string
    endpoint  string
    cdnURL    string
    urlSigner *sign.URLSigner // nil unless CloudFront signing is configured
    urlTTL    time.Duration
}

func NewS3Client(cfg *config.Config) (*S3Client, error) { ... }
func (c *S3Client) Upload(ctx context.Context, key string, data []byte, contentType string) error
func (c *S3Client) Download(ctx context.Context, key string) ([]byte, error)
func (c *S3Client) Delete(ctx context.Context, key string) error
func (c *S3Client) GetPublicURL(key string) string          // CloudFront URL; signed + expiring (CDN_URL_TTL, default 1 week) when signing is configured
func (c *S3Client) GetPresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error)
```

**HTTP clients** (`internal/platform/httpclient/`):
- Wrap external API calls with structured error handling.
- Use Zap for logging requests/responses.

### Event Bus

Built on [Watermill](https://watermill.io/) with Go channels (in-memory pub/sub).

**Publisher** (`pkg/eventbus/publisher.go`):

```go
type GoChannelEventBus struct {
    pubsub *gochannel.GoChannel
}

func (b *GoChannelEventBus) Publish(ctx context.Context, event ports.Event) error {
    msg := message.NewMessage(uuid.Must(uuid.NewV7()).String(), event.Data())
    return b.pubsub.Publish(event.EventName(), msg)
}
```

**Subscriber** (`pkg/eventbus/subscriber.go`):

```go
type GoChannelEventSubscriber struct {
    pubsub   *gochannel.GoChannel
    router   *message.Router
}

func (s *GoChannelEventSubscriber) Subscribe(handler ports.EventHandler) error {
    s.router.AddNoPublisherHandler(
        handler.EventName(),
        handler.EventName(),
        s.pubsub,
        func(msg *message.Message) error {
            return handler.Handle(context.Background(), msg.Payload)
        },
    )
    return nil
}
```

**Generic typed handler** (`pkg/eventbus/handler.go`):

```go
type TypedEventHandler[T ports.Event] struct {
    eventName string
    handleFn  func(ctx context.Context, event T) error
}

func NewTypedEventHandler[T ports.Event](eventName string, handleFn func(ctx context.Context, event T) error) *TypedEventHandler[T] {
    return &TypedEventHandler[T]{eventName: eventName, handleFn: handleFn}
}

func (h *TypedEventHandler[T]) Handle(ctx context.Context, payload []byte) error {
    var event T
    if err := json.Unmarshal(payload, &event); err != nil {
        return fmt.Errorf("failed to unmarshal event: %w", err)
    }
    return h.handleFn(ctx, event)
}
```

**Concurrency control** (`pkg/eventbus/product_lock_manager.go`):
- Per-product mutex locks to prevent race conditions during stock updates.

**Event flow**:

```
Service publishes event
  → EventBus.Publish(event)
    → Watermill GoChannel routes to subscribers
      → Subscriber 1: SSE notification handler
      → Subscriber 2: Stock update handler (with lock manager)
```

### SSE (Server-Sent Events)

Hub-based client management for real-time updates.

```go
// internal/platform/sse/hub.go

type Hub struct {
    mu      sync.RWMutex
    clients map[string]map[*Client]struct{}  // area → set of clients
}

func (h *Hub) Register(area string, client *Client)
func (h *Hub) Unregister(area string, client *Client)
func (h *Hub) NotifyArea(ctx context.Context, area string, eventType string, data interface{})
func (h *Hub) BroadcastAll(eventType string, data interface{})
```

Clients connect via SSE endpoints and receive real-time order updates, stock changes, etc.

### Cron Jobs

Scheduled tasks using a cron library for periodic operations.

---

## API Layer

### HTTP Handlers

Built on **Gin** framework. Each handler struct receives a service via constructor injection.

```go
// internal/platform/handler/order_handler.go

type OrderHandler struct {
    orderService *service.OrderService
}

func NewOrderHandler(orderService *service.OrderService) *OrderHandler {
    return &OrderHandler{orderService: orderService}
}

func (h *OrderHandler) CreateOrderHandler(c *gin.Context) {
    // 1. Bind request
    var req dto.CreateOrderRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
        return
    }

    // 2. Extract auth context
    userID, _ := c.Get("user_id")

    // 3. Call service
    result, err := h.orderService.CreateOrder(c.Request.Context(), &req, dto.UserDomain{ID: userID.(string)})
    if err != nil {
        // 4. Map domain errors to HTTP status codes
        handleError(c, err)
        return
    }

    // 5. Return response
    c.JSON(http.StatusCreated, result)
}
```

### Middleware Stack

```go
// Applied in order:

1. CORSMiddleware       // Handles preflight, allows configured origins
2. LoggerMiddleware     // Zap-based request logging (method, path, status, duration)
3. JWTAuthMiddleware    // Validates Bearer token, sets user_id + role_ids in context
4. AdminAPIKeyMiddleware // Validates X-API-Key for admin-only endpoints
5. RequirePermission    // Checks user roles against required permission
6. SSEMiddleware        // Sets headers for SSE connections (no buffering, keep-alive)
```

### Route Organization

Routes are organized by domain in `main.go`:

```go
// Public
router.POST("/api/auth/signin", userHandler.SignInHandler)
router.GET("/api/health", handler.HealthCheckHandler)

// Admin only (API key)
router.POST("/api/users", handler.AdminAPIKeyMiddleware(cfg), userHandler.CreateUserHandler)

// Protected (JWT + permissions)
api := router.Group("/api", handler.JWTAuthMiddleware(jwtService))
{
    // Orders
    api.POST("/orders", handler.RequirePermission(permissions.OrdersCreate), orderHandler.CreateOrderHandler)
    api.GET("/orders", handler.RequirePermission(permissions.OrdersRead), orderHandler.GetAllActiveOpenBillsHandler)
    api.PUT("/orders/:id", handler.RequirePermission(permissions.OrdersUpdate), orderHandler.UpdateOrderHandler)
    api.DELETE("/orders/:id", handler.RequirePermission(permissions.OrdersDelete), orderHandler.DeleteOrderHandler)

    // Products, Stock, Suppliers, Expenses, etc.
    // ...same pattern

    // SSE
    api.GET("/sse/commands/:area", handler.SSEMiddleware(), handler.RequirePermission(permissions.SSECommandsRead), sseHandler.StreamCommandsHandler)
}
```

---

## Permission System

Role-based access control (RBAC) with granular permissions.

```go
// internal/domain/permissions/role_permissions.go

type Permission string

const (
    OrdersCreate   Permission = "orders:create"
    OrdersRead     Permission = "orders:read"
    OrdersUpdate   Permission = "orders:update"
    OrdersDelete   Permission = "orders:delete"
    ProductsCreate Permission = "products:create"
    // ...resource:action pattern
)

// Role → Permission mapping
var rolePermissions = map[int][]Permission{
    1: {OrdersCreate, OrdersRead, OrdersUpdate, OrdersDelete, ProductsCreate, ...}, // Admin
    2: {OrdersCreate, OrdersRead, ...},                                               // Server
    3: {OrdersRead, ...},                                                              // Kitchen
}

func HasPermission(roleIDs []int, required Permission) bool
```

**Middleware integration**:

```go
func RequirePermission(required permissions.Permission) gin.HandlerFunc {
    return func(c *gin.Context) {
        roleIDs, _ := c.Get("role_ids")
        if !permissions.HasPermission(roleIDs.([]int), required) {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
            return
        }
        c.Next()
    }
}
```

---

## Error Handling

**BaseError** with stack traces:

```go
// internal/platform/shared/domain/error/base_error.go

type BaseError struct {
    code       string
    message    string
    fieldValue string     // What value caused the error
    cause      error      // Wrapped error
    stack      []uintptr  // Stack trace captured at creation
}

func NewBaseError(code, message, fieldValue string) *BaseError {
    // Captures stack trace via runtime.Callers()
}

func (e *BaseError) Error() string   { return e.message }
func (e *BaseError) GetCode() string { return e.code }
func (e *BaseError) Unwrap() error   { return e.cause }
```

**Error flow**:

```
Value Object validation fails → Aggregate-specific error (wraps BaseError)
  → Service propagates error
    → Handler maps error code/type to HTTP status code
```

---

## Configuration

Environment-based via `.env` file + `godotenv`.

```go
// internal/platform/config/config.go

type Config struct {
    JWTSecret                 string  // JWT signing key
    AdminAPIKey               string  // Admin API key
    ElectronicInvoiceURL      string  // External invoice service
    ElectronicInvoiceUser     string
    ElectronicInvoicePassword string
    StorageRegion             string  // AWS S3 storage
    StorageEndpoint           string  // optional; override for local MinIO
    StorageAccessKey          string  // optional; falls back to IAM role
    StorageSecret             string  // optional; falls back to IAM role
    StorageBucket             string
    CDNURL                    string  // CloudFront base URL for object URLs
    CDNKeyPairID              string  // optional; CloudFront signed-URL key pair ID
    CDNPrivateKey             string  // optional; inline PEM for CloudFront signing
    CDNPrivateKeyPath         string  // optional; PEM file path for CloudFront signing
    CDNURLTTL                 time.Duration // signed-URL lifetime (CDN_URL_TTL, default 1 week)
    OrganizationID            string  // Multi-tenant support
}

func NewConfig() (*Config, error) {
    // Reads from os.Getenv, returns error if required vars missing
}
```

**Required environment variables**: `JWT_SECRET`, `ADMIN_API_KEY`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`, `PORT`, `STORAGE_BUCKET` (plus optional `STORAGE_REGION`, `STORAGE_ACCESS_KEY`, `STORAGE_SECRET`, `STORAGE_ENDPOINT`, `CDN_URL`; for signed CloudFront links `CDN_KEY_PAIR_ID` + `CDN_PRIVATE_KEY` or `CDN_PRIVATE_KEY_PATH`, and optional `CDN_URL_TTL`), `ELECTRONIC_INVOICE_*`, `ORGANIZATION_ID`.

---

## Key Conventions

| Area | Convention |
|---|---|
| **IDs** | UUID v7 as strings (`uuid.Must(uuid.NewV7()).String()`) |
| **Money** | `decimal.Decimal` (shopspring) — never `float64` |
| **Timestamps** | `time.Time` with DB defaults |
| **Deletion** | Soft delete via `deleted_at` column |
| **Context** | Propagated through all layers for cancellation & transactions |
| **Errors** | Typed domain errors with codes + stack traces |
| **Testing** | Mockery-generated mocks for all ports |
| **Logging** | Zap (structured) for infrastructure, `slog` for domain events |
| **Framework** | Gin for HTTP |
| **ORM** | GORM with raw SQL for complex queries |
| **Events** | Watermill GoChannel (in-memory, swappable to Kafka/RabbitMQ) |
| **Auth** | JWT (Bearer token) + API key for admin |
| **Naming** | Aggregates: `Aggregate` struct; Repos: `XxxRepository`; Services: `XxxService`; Handlers: `XxxHandler` |
| **File structure** | One aggregate dir per bounded context; one file per value object |
| **DI** | Manual in `main.go` — no framework |
| **Transactions** | UnitOfWork pattern via context-embedded GORM `*gorm.DB` |
| **Concurrency** | `sync.RWMutex` in SSE hub; ProductLockManager for stock events |
