# FSIP2 Kubernetes Deployment

This directory contains Kubernetes manifests for deploying FSIP2 to a Kubernetes cluster.

## Overview

The deployment includes:
- **Namespace**: Isolated namespace for FSIP2 resources
- **ConfigMap**: Application and tenant configuration
- **Secret**: TLS certificates and sensitive credentials (template)
- **Deployment**: Application deployment with 2 replicas, health probes, and resource limits
- **Services**: ClusterIP, LoadBalancer, and headless services
- **Kustomization**: Kustomize configuration for customization

## Prerequisites

- Kubernetes cluster (v1.24+)
- kubectl configured to access your cluster
- Docker image `fsip2:1.0.0` available (push to registry or use local)
- (Optional) Kustomize for advanced customization
- FOLIO Okapi instance accessible from the cluster

## Quick Start

### 1. Build and Push Docker Image

```bash
# Build the image
make docker

# Tag for your registry
docker tag fsip2:1.0.0 your-registry.com/fsip2:1.0.0

# Push to registry
docker push your-registry.com/fsip2:1.0.0
```

### 2. Update Configuration

Edit `configmap.yaml` to configure:
- `okapiUrl`: Your FOLIO Okapi URL
- `tenants`: Your tenant configurations
- Other application settings

```bash
kubectl edit configmap fsip2-config -n fsip2
# Or edit the file directly and reapply
```

### 3. Deploy to Kubernetes

**Option A: Using kubectl**

```bash
# Deploy all resources
kubectl apply -f namespace.yaml
kubectl apply -f configmap.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# Or apply all at once
kubectl apply -f .
```

**Option B: Using Kustomize**

```bash
# Preview what will be deployed
kubectl kustomize .

# Deploy
kubectl apply -k .
```

### 4. Verify Deployment

```bash
# Check pod status
kubectl get pods -n fsip2

# Check services
kubectl get svc -n fsip2

# View logs
kubectl logs -n fsip2 -l app.kubernetes.io/name=fsip2 --tail=50

# Check health
kubectl port-forward -n fsip2 svc/fsip2 8081:8081
curl http://localhost:8081/admin/health
```

## Configuration

### ConfigMap

The `configmap.yaml` contains two configuration files:
- `config.yaml`: Bootstrap configuration (Okapi URL, ports, etc.)
- `tenant-config.yaml`: Tenant-specific settings

To update configuration:

```bash
# Edit the ConfigMap
kubectl edit configmap fsip2-config -n fsip2

# Restart pods to pick up changes
kubectl rollout restart deployment/fsip2 -n fsip2
```

### Secrets (Optional)

For TLS certificates or sensitive credentials:

1. Copy the secret template:
   ```bash
   cp secret.yaml secret-production.yaml
   ```

2. Add your secrets (base64 encoded):
   ```bash
   # Encode certificate
   cat tls.crt | base64

   # Encode key
   cat tls.key | base64
   ```

3. Update `secret-production.yaml` with encoded values

4. Apply the secret:
   ```bash
   kubectl apply -f secret-production.yaml
   ```

5. **Important**: Add `secret-production.yaml` to `.gitignore`

## Services

### ClusterIP Service (fsip2)

Internal service for cluster access:
- SIP2: `fsip2.fsip2.svc.cluster.local:6443`
- Metrics: `fsip2.fsip2.svc.cluster.local:8081`

### LoadBalancer Service (fsip2-external)

External access for SIP2 clients. This creates a cloud load balancer (AWS NLB, Azure LB, GCP LB).

Get the external IP:
```bash
kubectl get svc fsip2-external -n fsip2
```

**Note**: LoadBalancer type requires cloud provider support.

### Headless Service (fsip2-headless)

For direct pod-to-pod communication:
```bash
# Pods are accessible at:
fsip2-0.fsip2-headless.fsip2.svc.cluster.local
fsip2-1.fsip2-headless.fsip2.svc.cluster.local
```

## Scaling

### Manual Scaling

```bash
# Scale to 3 replicas
kubectl scale deployment fsip2 -n fsip2 --replicas=3

# Or edit deployment
kubectl edit deployment fsip2 -n fsip2
```

### Horizontal Pod Autoscaling (HPA)

```bash
# Create HPA based on CPU
kubectl autoscale deployment fsip2 -n fsip2 \
  --cpu-percent=70 \
  --min=2 \
  --max=10

# View HPA status
kubectl get hpa -n fsip2
```

## Resource Management

### Resource Limits

Default resources (per pod):
- **Requests**: 250m CPU, 256Mi memory
- **Limits**: 1000m CPU, 512Mi memory

Adjust in `deployment.yaml` based on your workload.

### Pod Disruption Budget (PDB)

Create a PDB to ensure availability during maintenance:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: fsip2-pdb
  namespace: fsip2
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: fsip2
```

```bash
kubectl apply -f pdb.yaml
```

## Monitoring

### Health Checks

The deployment includes three probes:
- **Liveness**: Restarts unhealthy containers
- **Readiness**: Removes unready pods from service
- **Startup**: Allows slow-starting containers extra time

### Prometheus Metrics

Metrics are exposed on port 8081 at `/metrics`. The pod annotations enable Prometheus auto-discovery.

Access metrics:
```bash
kubectl port-forward -n fsip2 svc/fsip2 8081:8081
curl http://localhost:8081/metrics
```

### Logs

View logs:
```bash
# All pods
kubectl logs -n fsip2 -l app.kubernetes.io/name=fsip2 --tail=100

# Specific pod
kubectl logs -n fsip2 fsip2-xxxxx-yyyyy

# Follow logs
kubectl logs -n fsip2 -l app.kubernetes.io/name=fsip2 -f

# Previous container logs
kubectl logs -n fsip2 fsip2-xxxxx-yyyyy --previous
```

## Troubleshooting

### Pod Not Starting

```bash
# Describe pod
kubectl describe pod -n fsip2 fsip2-xxxxx-yyyyy

# Check events
kubectl get events -n fsip2 --sort-by='.lastTimestamp'

# Check logs
kubectl logs -n fsip2 fsip2-xxxxx-yyyyy
```

### Configuration Issues

```bash
# View current ConfigMap
kubectl get configmap fsip2-config -n fsip2 -o yaml

# Test configuration
kubectl exec -n fsip2 fsip2-xxxxx-yyyyy -- cat /etc/fsip2/config.yaml
```

### Connectivity Issues

```bash
# Test health endpoint
kubectl exec -n fsip2 fsip2-xxxxx-yyyyy -- wget -qO- http://localhost:8081/admin/health

# Test SIP2 port
kubectl exec -n fsip2 fsip2-xxxxx-yyyyy -- nc -zv localhost 6443

# Test Okapi connectivity
kubectl exec -n fsip2 fsip2-xxxxx-yyyyy -- wget -qO- https://okapi.example.com
```

### Performance Issues

```bash
# View resource usage
kubectl top pods -n fsip2

# View resource requests/limits
kubectl describe pod -n fsip2 fsip2-xxxxx-yyyyy | grep -A 5 "Limits:"
```

## Updates and Rollback

### Rolling Update

```bash
# Update image
kubectl set image deployment/fsip2 fsip2=fsip2:1.0.1 -n fsip2

# Watch rollout
kubectl rollout status deployment/fsip2 -n fsip2

# View rollout history
kubectl rollout history deployment/fsip2 -n fsip2
```

### Rollback

```bash
# Rollback to previous version
kubectl rollout undo deployment/fsip2 -n fsip2

# Rollback to specific revision
kubectl rollout undo deployment/fsip2 -n fsip2 --to-revision=2
```

## Production Considerations

### High Availability

1. **Multiple Replicas**: Run at least 2-3 replicas across different nodes
2. **Pod Anti-Affinity**: Already configured to prefer different nodes
3. **Pod Disruption Budget**: Create PDB to ensure minimum availability
4. **Resource Limits**: Set appropriate CPU/memory limits

### Security

1. **Non-root User**: Deployment runs as UID 1001 (non-root)
2. **Read-only Root Filesystem**: Enabled for security
3. **Secrets Management**: Use Kubernetes secrets or external secret managers
4. **Network Policies**: Consider implementing network policies
5. **RBAC**: Use service accounts with minimal permissions

### Monitoring and Alerting

1. **Prometheus**: Configure Prometheus to scrape metrics
2. **Grafana**: Create dashboards for visualization
3. **Alerts**: Set up alerts for high error rates, pod restarts, resource usage

### Backup and Disaster Recovery

1. **Configuration**: Backup ConfigMaps and Secrets
2. **Manifests**: Store manifests in version control
3. **Disaster Recovery**: Document restoration procedures

## Customization with Kustomize

### Overlays

Create environment-specific overlays:

```bash
# Directory structure
deployments/kubernetes/
├── base/                    # Base manifests
│   ├── deployment.yaml
│   ├── service.yaml
│   └── kustomization.yaml
└── overlays/
    ├── development/
    │   ├── kustomization.yaml
    │   └── patch-replicas.yaml
    └── production/
        ├── kustomization.yaml
        └── patch-resources.yaml
```

Example overlay:
```yaml
# overlays/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
  - ../../base

replicas:
  - name: fsip2
    count: 5

images:
  - name: fsip2
    newName: your-registry.com/fsip2
    newTag: "1.0.0"

patchesStrategicMerge:
  - patch-resources.yaml
```

Deploy overlay:
```bash
kubectl apply -k overlays/production
```

## Cleanup

Remove all resources:

```bash
# Using kubectl
kubectl delete namespace fsip2

# Using kustomize
kubectl delete -k .
```

## Support

For issues and questions:
- GitHub Issues: [spokanepubliclibrary/fsip2](https://github.com/spokanepubliclibrary/fsip2/issues)
- FOLIO Community: [discuss.folio.org](https://discuss.folio.org)
