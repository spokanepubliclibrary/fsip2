# Edge-SIP2 (Go Implementation)

[![Go Version](https://img.shields.io/badge/go-1.23-blue.svg)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

A high-performance Go implementation of the SIP2 (Standard Interchange Protocol) server for FOLIO library management systems.

**Inspired by:** [folio-org/edge-sip2](https://github.com/folio-org/edge-sip2)
**Version:** 1.0.0
**Target FOLIO Release:** Quesnelia (Flower)

## Overview

Edge-SIP2 is a bridge application that connects self-service library kiosks with the FOLIO library management system using the SIP2 protocol. This Go implementation provides:

- Full SIP2 protocol support (13 message types)
- Multi-tenant configuration
- FOLIO API integration
- Configurable logging with PIN obfuscation
- Prometheus metrics
- Docker and Kubernetes deployment support

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
- ❌ Item Status Update (19/20)
- ✅ Login (93/94)
- ✅ SC/ACS Status (99/98)
- ✅ Resend (97/96)

See See [documentation/field_mapping.md](documentation/field_mapping.md) for comprehensive list of supported fields, configuration details, and data sources.


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
- Token-based authentication with caching
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
./bin/edge-sip2 -config config.yaml

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
  -t edge-sip2:1.0.0 .

docker tag edge-sip2:1.0.0 edge-sip2:latest
```

### Running with Docker

**Basic run (using example configs):**
```bash
# Using Makefile
make docker-run

# Manual run
docker run -d \
  --name edge-sip2 \
  -p 6443:6443 \
  -p 8081:8081 \
  -v $(pwd)/documentation/examples/basic.yaml:/etc/edge-sip2/config.yaml:ro \
  -v $(pwd)/documentation/examples/tenant-config.yaml:/etc/edge-sip2/tenant-config.yaml:ro \
  -v $(pwd)/log:/app/log \
  edge-sip2:latest
```

**Production run with custom configs:**
```bash
docker run -d \
  --name edge-sip2 \
  --restart unless-stopped \
  -p 6443:6443 \
  -p 8081:8081 \
  -v /etc/edge-sip2/config.yaml:/etc/edge-sip2/config.yaml:ro \
  -v /etc/edge-sip2/tenant-config.yaml:/etc/edge-sip2/tenant-config.yaml:ro \
  -v /var/log/edge-sip2:/app/log \
  --memory=512m \
  --cpus=2 \
  edge-sip2:latest
```

**With TLS enabled:**
```bash
docker run -d \
  --name edge-sip2 \
  -p 6443:6443 \
  -p 8081:8081 \
  -v $(pwd)/config.yaml:/etc/edge-sip2/config.yaml:ro \
  -v $(pwd)/tenant-config.yaml:/etc/edge-sip2/tenant-config.yaml:ro \
  -v $(pwd)/certs:/etc/edge-sip2/certs:ro \
  -v $(pwd)/log:/app/log \
  edge-sip2:latest
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
| User | edge-sip2 (non-root, UID 1001) |
| Working Directory | /app |
| Config Directory | /etc/edge-sip2 |
| Exposed Ports | 6443 (SIP2), 8081 (HTTP) |

### Volume Mounts

| Mount Point | Purpose | Required |
|-------------|---------|----------|
| /etc/edge-sip2/config.yaml | Bootstrap configuration | Yes |
| /etc/edge-sip2/tenant-config.yaml | Tenant configuration | Yes* |
| /app/log | Log files | No |
| /etc/edge-sip2/certs | TLS certificates | No |

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
docker logs edge-sip2

# Execute shell in container
docker exec -it edge-sip2 /bin/sh
```

### Testing the Docker Build

Before deploying to production, test the Docker build process:

```bash
# Test 1: Build using Makefile
make docker

# Test 2: Verify images were created
docker images | grep edge-sip2

# Test 3: Check image size (should be < 50MB)
docker images edge-sip2:latest --format "{{.Size}}"

# Test 4: Inspect image metadata
docker inspect edge-sip2:latest

# Test 5: Verify binary version
docker run --rm edge-sip2:latest /app/edge-sip2 -version

# Test 6: Check user permissions (should be non-root)
docker run --rm edge-sip2:latest id
# Expected: uid=1001(edge-sip2) gid=1001(edge-sip2)

# Test 7: Verify directory structure
docker run --rm edge-sip2:latest ls -la /app
docker run --rm edge-sip2:latest ls -la /etc/edge-sip2
```

**Troubleshooting Build Issues:**

If the build fails, check:
- Go module dependencies are accessible
- Network connectivity for downloading dependencies
- Sufficient disk space (at least 2GB free)
- Docker build cache (try `docker system prune` if needed)

## Kubernetes Deployment

Edge-SIP2 can be deployed to Kubernetes with production-ready manifests including health probes, resource limits, and auto-scaling support.

### Quick Start

```bash
# Option 1: Deploy with kubectl
kubectl apply -f deployments/kubernetes/

# Option 2: Deploy with Kustomize
kubectl apply -k deployments/kubernetes/

# Check deployment status
kubectl get pods -n edge-sip2
kubectl get svc -n edge-sip2

# View logs
kubectl logs -n edge-sip2 -l app.kubernetes.io/name=edge-sip2
```

### What's Included

The Kubernetes deployment provides:

- **Namespace**: Isolated `edge-sip2` namespace
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
kubectl edit configmap edge-sip2-config -n edge-sip2

# Update Okapi URL, tenant settings, etc.

# Restart pods to apply changes
kubectl rollout restart deployment/edge-sip2 -n edge-sip2
```

### Accessing the Service

**Internal Access (from within cluster):**
```
SIP2: edge-sip2.edge-sip2.svc.cluster.local:6443
Metrics: edge-sip2.edge-sip2.svc.cluster.local:8081
```

**External Access (LoadBalancer):**
```bash
# Get external IP
kubectl get svc edge-sip2-external -n edge-sip2

# Connect to SIP2
telnet <EXTERNAL-IP> 6443
```

### Scaling

```bash
# Manual scaling
kubectl scale deployment edge-sip2 -n edge-sip2 --replicas=5

# Auto-scaling (HPA)
kubectl autoscale deployment edge-sip2 -n edge-sip2 \
  --cpu-percent=70 --min=2 --max=10
```

### Monitoring

```bash
# Check health
kubectl port-forward -n edge-sip2 svc/edge-sip2 8081:8081
curl http://localhost:8081/admin/health

# View metrics
curl http://localhost:8081/metrics

# View logs
kubectl logs -n edge-sip2 -l app.kubernetes.io/name=edge-sip2 -f
```

For detailed Kubernetes deployment instructions, see [deployments/kubernetes/README.md](deployments/kubernetes/README.md).

## Testing

```bash
# Run unit tests
make test-unit

# Run integration tests
make test-integration

# Run e2e tests
make test-e2e

# Generate coverage report
make test-coverage
```

## Development

### Project Structure

```
edge-sip2/
├── cmd/edge-sip2/           # Main application
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
- `edge_sip2_command_duration_seconds` - Command processing duration
- `edge_sip2_invalid_message_errors_total` - Invalid message count
- `edge_sip2_request_errors_total` - Request error count
- `edge_sip2_response_errors_total` - Response error count

## Configuration Reference

### Bootstrap Configuration

- `port`: SIP2 server port (default: 6443)
- `okapiUrl`: FOLIO Okapi base URL
- `healthCheckPort`: Health check HTTP port (default: 8081)
- `tokenCacheCapacity`: Maximum cached tokens (default: 100)
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
- `notifyPatron`: Notify patron of payment via email/SMS (default: false)

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

Edge-SIP2 supports multiple tenants resolved by:
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

