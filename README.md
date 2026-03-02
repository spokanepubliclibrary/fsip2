# FSIP2 (Go Implementation)

[![Go Version](https://img.shields.io/badge/go-1.23-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

A high-performance, comprehensive, flexible and customizable Go implementation of the SIP2 (Standard Interchange Protocol) server for FOLIO library management systems.

**Inspired by:** [folio-org/edge-sip2](https://github.com/folio-org/edge-sip2)
**Version:** 1.0.0

## Overview

FSIP2 is a bridge application that connects self-service library kiosks with the FOLIO library management system using the SIP2 protocol. This Go implementation provides:

- Full SIP2 protocol support (13 message types)
- Multi-tenant configuration
- FOLIO API integration
- Configurable logging with PIN obfuscation
- Prometheus metrics
- Docker and Kubernetes deployment support

## Recent Improvements

### Security & Reliability Enhancements
- **Token Expiration Validation**: Automatic detection and refresh of expired FOLIO authentication tokens with 90-second safety buffer
- **Session Token Tracking**: Enhanced session management with token expiration timestamps
- **Improved Error Handling**: Better handling of authentication failures and token refresh scenarios

### Testing & Code Quality
- **Comprehensive Test Suite**: 86.9% overall coverage with most packages exceeding 85%
- **Test Separation**: Unit, integration, and E2E tests properly organized with build tags
- **100% Coverage Packages**: Cache, metrics, helpers, localization, types, customfields fully tested
- **Mock Infrastructure**: Complete mock FOLIO API for integration testing

### Configuration & Deployment
- **Multi-Source Configuration**: Support for file, HTTP, and S3-based configuration
- **Docker Optimization**: Alpine-based images with non-root user and minimal footprint
- **Kubernetes Ready**: Production-ready manifests with health probes and auto-scaling
- **Hot-Reload Support**: Configuration updates on service restart

## Features

### SIP2 Protocol Support
- ✅ Patron Status (23/24)
- ✅ Checkout (11/12)
- ✅ Checkin (09/10)
- ✅ Patron Information (63/64)
- ✅ Item Information (17/18)
- ✅ Renew (29/30)
- ✅ Renew All (65/66)
- ✅ End Patron Session (35/36)
- ✅ Fee Paid (37/38)
- ⚠️ Item Status Update (19/20) - Planned/Partial
- ✅ Login (93/94)
- ✅ SC/ACS Status (99/98)
- ✅ Resend (97/96)

See See [documentation/field_mapping.md](documentation/field_mapping.md) for comprehensive list of supported fields, configuration details, and data sources.


### Confirmed 100% Vendor Support (compared to folio-org/edge-sip2)
- Envisionware (Full suite) ✅
- Comprise (Terminals, SAM, SmartPay, SmartAlec) ✅
- MK Solutions (Full suite) ✅
- Illion (TalkingTech) iTiva Connect ✅


### Configuration
- YAML-based configuration
- Multi-source config loading (file, HTTP, S3)
- Hot-reload support (on restart)
- Per-tenant configuration
- Configurable message delimiters (CR, LF, CRLF)
- Character set support (IBM850, ISO-8859-1, UTF-8)

### Multi-Tenancy
- IP/subnet-based resolution
- Port-based resolution
- Location code resolution
- Username prefix resolution

### Security
- TLS/SSL support
- Token-based authentication with caching and automatic expiration validation
- Automatic token refresh for expired credentials
- PIN/password obfuscation in logs


#### Permissions
All permission required for fsip2:

```
    circulation.check-in-by-barcode.post
    circulation.check-out-by-barcode.post
    circulation.requests.collection.get
    search.instances.collection.get
    circulation.loans.collection.get
    configuration.entries.collection.get
    configuration.entries.item.get
    manualblocks.collection.get
    manualblocks.item.get
    accounts.collection.get
    accounts.item.get
    users.collection.get
    users.item.get
    patron-blocks.automated-patron-blocks.collection.get
    inventory.items.collection.get
    circulation.renew-by-barcode.post
    usergroups.collection.get
    users-bl.item.get
    usergroups.item.get
    usergroups.collection.get
    inventory-storage.holdings.item.get
    inventory-storage.service-points.item.get
    inventory.instances.item.get
    feefines.collection.get
    patron-pin.validate
    accounts.pay.post
    circulation.renew-by-id.post
    inventory.items.item.get
	  users.item.put (optional, see rollingRenewals feature for details)
```

### Monitoring
- Prometheus metrics
- Health check endpoint (/admin/health)
- Structured logging with Zap

## Quick Start

### Prerequisites

- Go 1.23 or higher
- Access to a FOLIO Okapi instance
- (Optional) Docker for containerized deployment

### Installation

```bash
# Clone the repository
git clone https://github.com/spokanepubliclibrary/fsip2.git
cd fsip2

# Install dependencies
make deps

# Build the application
make build
```

### Configuration

Create a configuration file (e.g., `config.yaml`):

```yaml
port: 6443
okapiUrl: https://okapi.example.com
healthCheckPort: 8081
logLevel: info

tenantConfigSources:
  - type: file
    path: ./tenant-config.yaml
```

See [documentation/examples/](documentation/examples/) for complete configuration examples.

### Running

```bash
# Run with configuration file
./bin/fsip2 -config config.yaml

# Or using make
make run
```

### Supported Messages and Configuration

Supported messages and configuration examples are stored in:
```
projectroot/
└── documentation/
    └── examples/
```

### Log Files

Log files are stored as `config-name.log` in:
```
projectroot/
└── log/
```

### Log Levels

Configure log level in the YAML file with the following options:

- **Debugging**: All messages (inbound and outbound) are logged. Service logins, message format and content, etc. with no obfuscation.
- **Full**: All messages (inbound and outbound) are logged. Excludes 93 & 94 login messages and PINs are obfuscated.
- **Patron**: Messages (63 inbound and 64 outbound) related to patron information are logged with PINs obfuscated.
- **None**: Default. No logging takes place.

### Configuration Options

Set configurations for:
- Supported fields
- Line termination (CR, LF, CRLF)
- Logging levels
- OKAPI endpoint and credentials
- Checksum validation
- Character encoding

Options can be found in the [documentation](documentation/) folder.

Example: ILS SIP Message delimiter configuration (CR vs CRLF)

## Docker Deployment

### Building the Docker Image

```bash
# Build using Makefile (recommended)
make docker

# Or build manually with version info
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  -t fsip2:1.0.0 .

docker tag fsip2:1.0.0 fsip2:latest
```

### Running with Docker

**Basic run (using example configs):**
```bash
# Using Makefile
make docker-run

# Manual run
docker run -d \
  --name fsip2 \
  -p 6443:6443 \
  -p 8081:8081 \
  -v $(pwd)/documentation/examples/basic.yaml:/etc/fsip2/config.yaml:ro \
  -v $(pwd)/documentation/examples/tenant-config.yaml:/etc/fsip2/tenant-config.yaml:ro \
  -v $(pwd)/log:/app/log \
  fsip2:latest
```

**Production run with custom configs:**
```bash
docker run -d \
  --name fsip2 \
  --restart unless-stopped \
  -p 6443:6443 \
  -p 8081:8081 \
  -v /etc/fsip2/config.yaml:/etc/fsip2/config.yaml:ro \
  -v /etc/fsip2/tenant-config.yaml:/etc/fsip2/tenant-config.yaml:ro \
  -v /var/log/fsip2:/app/log \
  --memory=512m \
  --cpus=2 \
  fsip2:latest
```

**With TLS enabled:**
```bash
docker run -d \
  --name fsip2 \
  -p 6443:6443 \
  -p 8081:8081 \
  -v $(pwd)/config.yaml:/etc/fsip2/config.yaml:ro \
  -v $(pwd)/tenant-config.yaml:/etc/fsip2/tenant-config.yaml:ro \
  -v $(pwd)/certs:/etc/fsip2/certs:ro \
  -v $(pwd)/log:/app/log \
  fsip2:latest
```

### Using Docker Compose

A complete docker-compose.yaml example is available in `documentation/examples/`:

```bash
cd documentation/examples

# Start the service
docker-compose up -d

# View logs
docker-compose logs -f

# Check health
curl http://localhost:8081/admin/health

# Stop the service
docker-compose down
```

### Docker Image Details

| Property | Value |
|----------|-------|
| Base Image | alpine:3.19 |
| User | fsip2 (non-root, UID 1001) |
| Working Directory | /app |
| Config Directory | /etc/fsip2 |
| Exposed Ports | 6443 (SIP2), 8081 (HTTP) |

### Volume Mounts

| Mount Point | Purpose | Required |
|-------------|---------|----------|
| /etc/fsip2/config.yaml | Bootstrap configuration | Yes |
| /etc/fsip2/tenant-config.yaml | Tenant configuration | Yes* |
| /app/log | Log files | No |
| /etc/fsip2/certs | TLS certificates | No |

*Required if using file-based tenant configuration

### Verifying the Container

```bash
# Check container status
docker ps

# Check health endpoint
curl http://localhost:8081/admin/health

# Check readiness
curl http://localhost:8081/admin/ready

# View metrics
curl http://localhost:8081/metrics

# Test SIP2 connection
telnet localhost 6443

# View container logs
docker logs fsip2

# Execute shell in container
docker exec -it fsip2 /bin/sh
```

### Testing the Docker Build

Before deploying to production, test the Docker build process:

```bash
# Test 1: Build using Makefile
make docker

# Test 2: Verify images were created
docker images | grep fsip2

# Test 3: Check image size (should be < 50MB)
docker images fsip2:latest --format "{{.Size}}"

# Test 4: Inspect image metadata
docker inspect fsip2:latest

# Test 5: Verify binary version
docker run --rm fsip2:latest /app/fsip2 -version

# Test 6: Check user permissions (should be non-root)
docker run --rm fsip2:latest id
# Expected: uid=1001(fsip2) gid=1001(fsip2)

# Test 7: Verify directory structure
docker run --rm fsip2:latest ls -la /app
docker run --rm fsip2:latest ls -la /etc/fsip2
```

**Troubleshooting Build Issues:**

If the build fails, check:
- Go module dependencies are accessible
- Network connectivity for downloading dependencies
- Sufficient disk space (at least 2GB free)
- Docker build cache (try `docker system prune` if needed)

## Kubernetes Deployment

FSIP2 can be deployed to Kubernetes with production-ready manifests including health probes, resource limits, and auto-scaling support.

### Quick Start

```bash
# Option 1: Deploy with kubectl
kubectl apply -f deployments/kubernetes/

# Option 2: Deploy with Kustomize
kubectl apply -k deployments/kubernetes/

# Check deployment status
kubectl get pods -n fsip2
kubectl get svc -n fsip2

# View logs
kubectl logs -n fsip2 -l app.kubernetes.io/name=fsip2
```

### What's Included

The Kubernetes deployment provides:

- **Namespace**: Isolated `fsip2` namespace
- **Deployment**: 2 replicas with rolling updates, health probes, resource limits
- **Services**: ClusterIP (internal), LoadBalancer (external), and headless services
- **ConfigMap**: Application and tenant configuration
- **Security**: Non-root containers, read-only root filesystem, pod security standards
- **Monitoring**: Prometheus metrics endpoint, health checks
- **High Availability**: Pod anti-affinity, pod disruption budgets

### Configuration

Edit the ConfigMap to configure your deployment:

```bash
# Edit configuration
kubectl edit configmap fsip2-config -n fsip2

# Update Okapi URL, tenant settings, etc.

# Restart pods to apply changes
kubectl rollout restart deployment/fsip2 -n fsip2
```

### Accessing the Service

**Internal Access (from within cluster):**
```
SIP2: fsip2.fsip2.svc.cluster.local:6443
Metrics: fsip2.fsip2.svc.cluster.local:8081
```

**External Access (LoadBalancer):**
```bash
# Get external IP
kubectl get svc fsip2-external -n fsip2

# Connect to SIP2
telnet <EXTERNAL-IP> 6443
```

### Scaling

```bash
# Manual scaling
kubectl scale deployment fsip2 -n fsip2 --replicas=5

# Auto-scaling (HPA)
kubectl autoscale deployment fsip2 -n fsip2 \
  --cpu-percent=70 --min=2 --max=10
```

### Monitoring

```bash
# Check health
kubectl port-forward -n fsip2 svc/fsip2 8081:8081
curl http://localhost:8081/admin/health

# View metrics
curl http://localhost:8081/metrics

# View logs
kubectl logs -n fsip2 -l app.kubernetes.io/name=fsip2 -f
```

For detailed Kubernetes deployment instructions, see [deployments/kubernetes/README.md](deployments/kubernetes/README.md).

## Testing

FSIP2 has a comprehensive test suite with multiple testing layers to ensure reliability and correctness.

### Test Coverage

**Current Coverage: 86.9%**

**Package-Level Coverage:**
- `internal/cache`: 100.0% - Full coverage of token caching
- `internal/helpers`: 100.0% - Helper utilities
- `internal/localization`: 100.0% - Localization support
- `internal/metrics`: 100.0% - Complete metrics instrumentation
- `internal/sip2/customfields`: 100.0% - Custom field handling
- `internal/types`: 100.0% - Session and type definitions
- `internal/sip2/protocol`: 98.1% - Protocol primitives (charset, datetime, delimiters)
- `internal/folio/models`: 97.3% - FOLIO API data models
- `internal/sip2/mediatype`: 96.2% - Media type mapping
- `internal/health`: 90.9% - Health check endpoints
- `internal/logging`: 91.5% - Comprehensive logging tests
- `internal/renewal`: 89.5% - Rolling renewal logic
- `internal/sip2/builder`: 88.0% - SIP2 message building
- `internal/folio`: 88.2% - FOLIO API integration
- `internal/server`: 87.7% - Server and connection handling
- `internal/config`: 87.1% - Configuration management
- `internal/sip2/parser`: 86.6% - SIP2 message parsing
- `internal/tenant`: 87.8% - Multi-tenant resolution
- `internal/handlers`: 84.1% - Message handlers

### Test Types

**Unit Tests** - Fast, isolated component tests:
```bash
make test-unit
```

**Integration Tests** - Tests with mock FOLIO API:
```bash
make test-integration
```

**End-to-End Tests** - Full SIP2 protocol flow tests:
```bash
make test-e2e
```

**All Tests** - Run unit, integration, and E2E together:
```bash
make test-all
```

**Coverage Report** - Generate HTML coverage report:
```bash
make test-coverage
# Opens coverage.html in browser
```

### Test Organization

```
fsip2/
├── internal/                    # Unit tests co-located with source
│   ├── cache/                   # *_test.go alongside source files
│   ├── handlers/                # Handler tests with mock helpers
│   └── ...
└── tests/
    ├── e2e/                     # End-to-end SIP2 protocol flow tests
    ├── integration/             # Integration tests with mock FOLIO server
    ├── mocks/                   # Mock FOLIO API server
    ├── fixtures/                # JSON test fixtures (users, items, loans, etc.)
    └── testutil/                # Shared test utilities
```

Tests are organized using Go build tags:
- Unit tests: No build tags required (run with `make test-unit`)
- Integration tests: `//go:build integration` (run with `make test-integration`)
- E2E tests: `//go:build e2e` (run with `make test-e2e`)

This separation ensures fast feedback during development while maintaining comprehensive test coverage.

### Key Features Tested

- **Token Expiration & Refresh**: Automatic detection and refresh of expired FOLIO authentication tokens
- **Multi-Tenant Resolution**: IP-based, port-based, and username-prefix tenant routing
- **SIP2 Protocol**: All 13 message types with field validation
- **FOLIO Integration**: API client with retry logic and error handling
- **Configuration**: Hot-reload, multi-source config loading (file, HTTP, S3)
- **Rolling Renewals**: Automatic patron expiration extension
- **Metrics & Monitoring**: Prometheus metrics and health endpoints
- **TLS/Connection Handling**: Secure connection setup and lifecycle management

## Development

### Project Structure

```
fsip2/
├── cmd/fsip2/               # Main application
├── internal/                # Private application code
│   ├── config/              # Configuration management
│   ├── sip2/                # SIP2 protocol
│   ├── handlers/            # Message handlers
│   ├── folio/               # FOLIO API integration
│   ├── tenant/              # Multi-tenant support
│   └── ...
├── test/                    # Tests
├── documentation/           # Documentation
├── deployments/             # Deployment configs
└── log/                     # Log files
```

### Building

```bash
make build              # Build binary
make install            # Install to GOPATH/bin
make clean              # Clean build artifacts
```

### Code Style

This project follows standard Go conventions:

- **Formatting**: `gofmt` and `go vet` (run via `make lint`)
- **Naming**: PascalCase for exported identifiers, camelCase for unexported; constructors use `NewXxx()` pattern; method receivers use short variable names
- **Error handling**: Wrap errors with context using `fmt.Errorf("...: %w", err)`
- **Documentation**: Package-level `doc.go` files; exported functions commented starting with the function name
- **Imports**: Standard library, then third-party, separated by blank lines
- **Tests**: Table-driven tests with `testify`; integration and E2E tests separated by build tags (`integration`, `e2e`)

Run formatting and vet checks:
```bash
make lint
```

## Monitoring

### Health Check

```bash
curl http://localhost:8081/admin/health
```

### Metrics

```bash
curl http://localhost:8081/metrics
```

Available metrics:
- `fsip2_connections_total` - Total SIP2 connections established
- `fsip2_connections_active` - Current active SIP2 connections
- `fsip2_connection_duration_seconds` - Duration of SIP2 connections
- `fsip2_connection_errors_total` - Total SIP2 connection errors
- `fsip2_messages_total` - Total SIP2 messages by type and tenant
- `fsip2_message_duration_seconds` - SIP2 message processing duration
- `fsip2_message_errors_total` - Total SIP2 message errors by type
- `fsip2_login_attempts_total` - Total login attempts
- `fsip2_login_success_total` - Total successful logins
- `fsip2_login_failures_total` - Total failed logins
- `fsip2_checkout_total` - Total checkout requests
- `fsip2_checkout_success_total` - Total successful checkouts
- `fsip2_checkout_failures_total` - Total failed checkouts
- `fsip2_checkin_total` - Total checkin requests
- `fsip2_checkin_success_total` - Total successful checkins
- `fsip2_checkin_failures_total` - Total failed checkins
- `fsip2_renew_total` - Total renewal requests
- `fsip2_renew_success_total` - Total successful renewals
- `fsip2_renew_failures_total` - Total failed renewals
- `fsip2_tenant_resolutions_total` - Total tenant resolutions by phase and resolver
- `fsip2_tenant_resolution_errors_total` - Total tenant resolution errors
- `fsip2_sessions_created_total` - Total SIP2 sessions created
- `fsip2_sessions_ended_total` - Total SIP2 sessions ended
- `fsip2_sessions_active` - Current active SIP2 sessions
- `folio_requests_total` - Total FOLIO API requests by endpoint
- `folio_request_duration_seconds` - FOLIO API request duration
- `folio_request_errors_total` - Total FOLIO API errors by endpoint

## Configuration Reference

### Bootstrap Configuration

- `port`: SIP2 server port (default: 6443)
- `okapiUrl`: FOLIO Okapi base URL
- `healthCheckPort`: Health check HTTP port (default: 8081)
- `tokenCacheCapacity`: Maximum cached tokens (default: 100)
- `tokenCacheTTL`: Token cache time-to-live with automatic expiration validation
- `scanPeriod`: Config reload interval in ms (default: 300000)

### Tenant Configuration

- `tenant`: FOLIO tenant ID
- `errorDetectionEnabled`: Enable checksum validation
- `messageDelimiter`: Message terminator (\r, \n, \r\n)
- `fieldDelimiter`: Field separator (usually |)
- `charset`: Character encoding (IBM850, ISO-8859-1, UTF-8)
- `logLevel`: Logging level (Debugging, Full, Patron, None)

#### Fee/Fine Payment Settings (Message 37)

- `acceptBulkPayment`: Enable bulk payment fallback (default: false)
  - When true: If account ID not found/provided, distributes payment evenly across all eligible open accounts
  - When false: Returns error if account ID not found
- `paymentMethod`: Default payment method (default: "Credit card")
  - Common values: "Credit card", "Cash", "Check", "Debit card"
- `notifyPatron`: Notify patron of payment via email (MOD-Notice) (default: false)

**Note:** The CP (Location Code) field in login message (93) must contain the UUID of the service point, not a location code string.

#### Rolling Renewals (Automated Patron Expiration Extension)

Rolling renewals automatically extend a patron's expiration date when they authenticate via SIP2 messages 23 (Patron Status) or 63 (Patron Information). This feature helps maintain active patron accounts without manual intervention.

**Configuration:**
```yaml
rollingRenewals:
  enabled: true                # Master switch (default: false)
  renewWithin: 6M              # Renew if expiration within 6 months (M/D/Y)
  extendFor: 6Y                # Extend expiration to 6 years from today (M/D/Y)
  extendExpired: true          # Extend expired accounts from today (default: false)
  dryRun: false                # Log actions without updating (default: false)
  selectPatrons: true          # Only renew specific patron groups (default: false)
  allowedPatrons:              # List of patron group UUIDs (required if selectPatrons=true)
    - bb7e08fe-63e9-3d0a-91c7-9448c1fe92bb
    - uuid-2
```

**Required FOLIO Permissions:**
- `users.item.get` - To fetch user records
- `users.item.put` - To update user expiration dates

**Key Features:**
- **Automatic Extension**: Extends expiration dates when patrons authenticate
- **Configurable Windows**: Define when to renew (`renewWithin`) and extension period (`extendFor`)
- **Patron Group Filtering**: Optionally limit renewals to specific patron groups
- **Expired Account Handling**: Choose whether to renew expired accounts
- **Dry-Run Mode**: Test renewal logic without modifying data
- **Non-Blocking**: Renewal errors never block SIP responses
- **Structured Logging**: Comprehensive logs for monitoring and debugging

**Duration Format:**
- `M` - Months (e.g., `6M` = 6 months)
- `D` - Days (e.g., `30D` = 30 days)
- `Y` - Years (e.g., `1Y` = 1 year)
- Case-insensitive: `6M`, `6m`, `6Y`, `6y` all valid

**Example Scenarios:**
1. **Standard Renewal**: Patron expiring in 4 months → Extended to 6 years from today
2. **Expired Account**: Patron expired 2 months ago + `extendExpired: true` → Extended to 6 years from today
3. **Outside Window**: Patron expiring in 8 months with `renewWithin: 6M` → No renewal
4. **Selective Renewal**: Only undergrad/faculty groups renewed, guests excluded

See [documentation/configuration.md](documentation/configuration.md) for complete reference.

## Multi-Tenant Setup

FSIP2 supports multiple tenants resolved by:
- Client IP address (CIDR ranges)
- Connection port
- SIP2 location code
- Username prefix

See [documentation/multi-tenant.md](documentation/multi-tenant.md) for setup guide.

## Troubleshooting

### Connection Issues
- Verify port is accessible: `telnet localhost 6443`
- Check TLS configuration if enabled
- Review logs in `log/` directory

### Authentication Failures
- Verify Okapi URL is correct
- Check tenant credentials
- Review token cache settings

### Message Parsing Errors
- Verify character encoding matches SIP2 client
- Check message delimiter configuration
- Enable Debugging log level for detailed message inspection

## Contributing

Contributions are welcome! 

## License

Apache 2.0

## Support

For issues and questions:
- GitHub Issues: [spokanepubliclibrary/fsip2](https://github.com/spokanepubliclibrary/fsip2/issues)
- FOLIO Community: [discuss.folio.org](https://discuss.folio.org)

