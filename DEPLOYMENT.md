# PeerDB Exporter Deployment Guide

## Docker Deployment

### Method 1: Environment Variables
```bash
docker run -d \
  --name peerdb-exporter \
  -e PEERDB_HOST=your-db-host \
  -e PEERDB_PORT=5432 \
  -e PEERDB_USERNAME=your-username \
  -e PEERDB_PASSWORD=your-password \
  -e PEERDB_DATABASE=peerdb_stats \
  -p 8001:8080 \
  peerdb-exporter:latest
```

### Method 2: Environment File
```bash
# 1. Copy the example environment file
cp .env.example .env

# 2. Edit .env with your actual values
# 3. Run with environment file
docker run -d \
  --name peerdb-exporter \
  --env-file .env \
  -p 8001:8080 \
  peerdb-exporter:latest
```

### Method 3: Docker Compose
```yaml
# docker-compose.yml
version: '3.8'
services:
  peerdb-exporter:
    image: peerdb-exporter:latest
    ports:
      - "8001:8080"
    environment:
      PEERDB_HOST: your-db-host
      PEERDB_PORT: 5432
      PEERDB_USERNAME: your-username
      PEERDB_PASSWORD: your-password
      PEERDB_DATABASE: peerdb_stats
    # Or use env_file:
    # env_file: .env
```

## Kubernetes Deployment

### Method 1: Helm with Custom Values
```bash
# 1. Update custom-values.yaml with your configuration
# 2. Deploy
helm install peerdb-exporter ./peerdb_exporter -f custom-values.yaml
```

### Method 2: Helm with Command Line Overrides
```bash
helm install peerdb-exporter ./peerdb_exporter \
  --set env.PEERDB_HOST=your-db-host \
  --set env.PEERDB_USERNAME=your-username \
  --set env.PEERDB_PASSWORD=your-password \
  --set image.repository=your-registry/peerdb-exporter \
  --set image.tag=v1.0.0
```

### Method 3: Using Kubernetes Secrets (Recommended)
```bash
# 1. Create the secret
kubectl create secret generic peerdb-secrets \
  --from-literal=PEERDB_USERNAME=your-username \
  --from-literal=PEERDB_PASSWORD=your-password

# 2. Deploy with values that reference the secret
helm install peerdb-exporter ./peerdb_exporter \
  --set env.PEERDB_HOST=your-db-host \
  --set env.PEERDB_PORT=5432 \
  --set env.PEERDB_DATABASE=peerdb_stats \
  --set image.repository=your-registry/peerdb-exporter \
  --set image.tag=v1.0.0
```

### Method 4: Environment-specific Values Files
```bash
# For different environments
helm install peerdb-exporter-dev ./peerdb_exporter -f values-dev.yaml
helm install peerdb-exporter-prod ./peerdb_exporter -f values-prod.yaml
```

## Building and Pushing Docker Image

### Build with Build Args (if needed during build time)
```bash
docker build \
  --build-arg PEERDB_HOST=build-time-host \
  -t peerdb-exporter:latest .
```

### Build and Push
```bash
# Build
docker build -t your-registry/peerdb-exporter:v1.0.0 .

# Push
docker push your-registry/peerdb-exporter:v1.0.0
```

## Environment Variables Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| PEERDB_HOST | Yes | - | Database host for PeerDB |
| PEERDB_PORT | Yes | - | Database port for PeerDB |
| PEERDB_USERNAME | Yes | - | Database username |
| PEERDB_PASSWORD | Yes | - | Database password |
| PEERDB_DATABASE | No | peerdb_stats | Database name |

## Security Best Practices

1. **Never hardcode passwords** in Dockerfiles or values.yaml
2. **Use Kubernetes Secrets** for sensitive data in K8s deployments
3. **Use environment files** (.env) for Docker that are not committed to git
4. **Rotate credentials** regularly
5. **Use minimal database permissions** (read-only user recommended)
