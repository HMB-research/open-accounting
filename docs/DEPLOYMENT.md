# Deployment Guide

This guide covers deploying Open Accounting to production environments.

## Prerequisites

- Docker and Docker Compose (recommended)
- OR: Go 1.22+, Node.js 22+, PostgreSQL 16+
- A domain name with SSL certificate
- Minimum 2GB RAM, 10GB storage

## Environment Variables

### Backend API

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `DATABASE_URL` | Yes | PostgreSQL connection string | `postgres://user:pass@host:5432/db?sslmode=require` |
| `JWT_SECRET` | Yes | Secret key for JWT signing (min 32 chars) | `your-super-secret-key-min-32-chars` |
| `PORT` | No | API server port | `8080` |
| `ALLOWED_ORIGINS` | Yes* | CORS allowed origins (comma-separated) | `https://app.example.com,https://admin.example.com` |
| `CORS_DEBUG` | No | Enable verbose CORS logging | `true` |
| `LOG_LEVEL` | No | Log verbosity (trace, debug, info, warn, error) | `debug` |
| `DEMO_RESET_SECRET` | No | Secret key for demo reset endpoint | `your-reset-secret` |

*Required for production deployments where frontend and API are on different domains.

### Frontend

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `PUBLIC_API_URL` | Yes | Backend API URL (must include `https://`) | `https://api.example.com` |

> **Note:** If `PUBLIC_API_URL` is set without a protocol (e.g., `api.example.com`), the frontend will automatically prepend `https://`.

## Docker Deployment

### 1. Production Docker Compose

Create a `docker-compose.prod.yml`:

```yaml
version: '3.8'

services:
  api:
    image: ghcr.io/hmb-research/open-accounting:latest
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - JWT_SECRET=${JWT_SECRET}
      - PORT=8080
      - ALLOWED_ORIGINS=${ALLOWED_ORIGINS}
    ports:
      - "8080:8080"
    depends_on:
      - db
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  db:
    image: postgres:16-alpine
    environment:
      - POSTGRES_USER=${DB_USER}
      - POSTGRES_PASSWORD=${DB_PASSWORD}
      - POSTGRES_DB=${DB_NAME}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

  migrate:
    image: ghcr.io/hmb-research/open-accounting:latest
    command: ["./migrate", "-db", "${DATABASE_URL}", "-path", "migrations", "-direction", "up"]
    depends_on:
      - db

volumes:
  postgres_data:
```

### 2. Create Environment File

```bash
# .env.prod
DATABASE_URL=postgres://openaccounting:SECURE_PASSWORD@db:5432/openaccounting?sslmode=disable
JWT_SECRET=your-production-jwt-secret-minimum-32-characters
ALLOWED_ORIGINS=https://your-domain.com

DB_USER=openaccounting
DB_PASSWORD=SECURE_PASSWORD
DB_NAME=openaccounting
```

### 3. Deploy

```bash
# Pull latest images
docker-compose -f docker-compose.prod.yml pull

# Run migrations
docker-compose -f docker-compose.prod.yml run --rm migrate

# Start services
docker-compose -f docker-compose.prod.yml up -d

# Check logs
docker-compose -f docker-compose.prod.yml logs -f api
```

## Nginx Reverse Proxy

Example Nginx configuration with SSL:

```nginx
server {
    listen 80;
    server_name api.example.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.example.com;

    ssl_certificate /etc/letsencrypt/live/api.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.example.com/privkey.pem;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}
```

## Database Considerations

### PostgreSQL Configuration

For production, tune these settings in `postgresql.conf`:

```ini
# Memory
shared_buffers = 256MB          # 25% of RAM
effective_cache_size = 768MB    # 75% of RAM
work_mem = 16MB

# Connections
max_connections = 100

# WAL
wal_level = replica
max_wal_senders = 3

# Logging
log_statement = 'mod'
log_min_duration_statement = 1000
```

### Backup Strategy

```bash
# Daily backup script
#!/bin/bash
DATE=$(date +%Y%m%d)
pg_dump $DATABASE_URL | gzip > /backups/openaccounting_$DATE.sql.gz

# Keep last 30 days
find /backups -name "*.sql.gz" -mtime +30 -delete
```

### Connection Pooling

For high-traffic deployments, use PgBouncer:

```ini
# pgbouncer.ini
[databases]
openaccounting = host=db port=5432 dbname=openaccounting

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
auth_type = md5
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 20
```

## Kubernetes Deployment

Basic Kubernetes manifests:

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: open-accounting-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: open-accounting-api
  template:
    metadata:
      labels:
        app: open-accounting-api
    spec:
      containers:
      - name: api
        image: ghcr.io/hmb-research/open-accounting:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: open-accounting-secrets
              key: database-url
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: open-accounting-secrets
              key: jwt-secret
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: open-accounting-api
spec:
  selector:
    app: open-accounting-api
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
```

## Railway Deployment

Railway provides a simple PaaS deployment option. The project includes Railway configuration files.

### Services Setup

Deploy two separate services:

1. **Backend API** (`open-accounting-api`)
   - Root directory: `/` (uses Go backend)
   - Environment variables:
     ```
     DATABASE_URL=<from Railway PostgreSQL>
     JWT_SECRET=<generate secure 32+ char secret>
     ALLOWED_ORIGINS=https://your-frontend.up.railway.app
     ```

2. **Frontend** (`open-accounting`)
   - Root directory: `/frontend`
   - Environment variables:
     ```
     PUBLIC_API_URL=https://your-api.up.railway.app
     ```

3. **PostgreSQL Database**
   - Add PostgreSQL plugin from Railway dashboard
   - Copy `DATABASE_URL` to API service

### Demo Mode (Optional)

For demo deployments with sample data and hourly reset:

```
DEMO_MODE=true
DEMO_RESET_SECRET=<your-secret-key>
```

Trigger reset via: `POST /api/demo/reset` with `X-Demo-Secret` header.

## CORS Troubleshooting

If you encounter CORS errors like:

```
Access to fetch at 'https://api.example.com/...' has been blocked by CORS policy:
No 'Access-Control-Allow-Origin' header is present on the requested resource.
```

### Common Causes & Solutions

1. **Missing ALLOWED_ORIGINS**
   - Ensure `ALLOWED_ORIGINS` includes your frontend URL
   - Example: `ALLOWED_ORIGINS=https://app.example.com`

2. **Multiple Origins**
   - Use comma-separated values (no spaces around commas)
   - Example: `ALLOWED_ORIGINS=https://app.example.com,https://staging.example.com`

3. **Protocol Mismatch**
   - Ensure both URLs use `https://`
   - `http://` and `https://` are treated as different origins

4. **Trailing Slashes**
   - Don't include trailing slashes in origins
   - Correct: `https://app.example.com`
   - Wrong: `https://app.example.com/`

### Debugging

Enable verbose CORS logging:
```
CORS_DEBUG=true
```

Check API logs at startup for:
```
CORS configuration allowed_origins=["http://localhost:5173","https://app.example.com"]
```

### Verify Configuration

```bash
# Test preflight request
curl -X OPTIONS https://api.example.com/api/v1/auth/login \
  -H "Origin: https://app.example.com" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type" \
  -v

# Should return headers including:
# Access-Control-Allow-Origin: https://app.example.com
# Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS
```

## Security Checklist

- [ ] Use strong, unique `JWT_SECRET` (min 32 characters)
- [ ] Enable SSL/TLS for all connections
- [ ] Use `sslmode=require` for database connections
- [ ] Configure firewall rules (only expose ports 80/443)
- [ ] Enable database connection encryption
- [ ] Set up automated backups
- [ ] Configure log rotation
- [ ] Use secrets management (Vault, AWS Secrets Manager, etc.)
- [ ] Enable rate limiting at reverse proxy level
- [ ] Regular security updates for OS and dependencies

## Monitoring

### Health Check Endpoint

```
GET /health
```

Returns `200 OK` with body `OK` when healthy.

### Recommended Metrics

- Request latency (p50, p95, p99)
- Error rate (4xx, 5xx responses)
- Database connection pool usage
- Memory and CPU usage

### Logging

The API outputs structured logs. Recommended log aggregation:
- ELK Stack (Elasticsearch, Logstash, Kibana)
- Grafana Loki
- CloudWatch Logs (AWS)
