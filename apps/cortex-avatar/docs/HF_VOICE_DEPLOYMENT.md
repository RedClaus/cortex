---
project: Cortex
component: Infra
phase: Ideation
date_created: 2026-02-07T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-07T15:26:36.472679
---

# HF Voice Pipeline - Deployment Guide

**Version:** 2.4.0
**Last Updated:** 2026-02-07
**Audience:** DevOps, System Administrators

---

## Table of Contents

1. [Deployment Options](#deployment-options)
2. [Docker Deployment](#docker-deployment)
3. [Production Configuration](#production-configuration)
4. [Monitoring & Logging](#monitoring--logging)
5. [Scaling Strategies](#scaling-strategies)
6. [Security Hardening](#security-hardening)
7. [Troubleshooting Production Issues](#troubleshooting-production-issues)

---

## Deployment Options

### Option 1: Docker Compose (Recommended)

**Best for:** Single-server deployments, development, small teams

- ✅ Easy setup and management
- ✅ Consistent environment
- ✅ Built-in health checks
- ❌ Limited scalability

### Option 2: Kubernetes

**Best for:** Large-scale deployments, high availability, auto-scaling

- ✅ Auto-scaling and load balancing
- ✅ High availability
- ✅ Rolling updates
- ❌ Complex setup

### Option 3: Systemd Service

**Best for:** Simple server deployments, existing infrastructure

- ✅ Native Linux integration
- ✅ Auto-restart on failure
- ✅ Resource limiting
- ❌ Manual dependency management

---

## Docker Deployment

### Step 1: Build Docker Image

```dockerfile
# Dockerfile
FROM python:3.11-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy requirements
COPY requirements.txt requirements_mac.txt ./

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt
RUN pip install --no-cache-dir -r requirements_mac.txt

# Copy application code
COPY . .

# Download models (optional - can be done at runtime)
RUN python -c "from transformers import pipeline; \
    pipeline('automatic-speech-recognition', model='openai/whisper-large-v3-turbo'); \
    pipeline('text-to-speech', model='myshell-ai/MeloTTS-English')"

# Expose port
EXPOSE 8899

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -f http://localhost:8899/health || exit 1

# Run service
CMD ["python", "service.py"]
```

### Step 2: Docker Compose Configuration

```yaml
# docker-compose.yml
version: '3.8'

services:
  hf-voice:
    build: .
    container_name: cortex-hf-voice
    ports:
      - "8899:8899"
    environment:
      - LOG_LEVEL=INFO
      - MAX_WORKERS=4
      - TIMEOUT=30
    volumes:
      - ./models:/app/models
      - ./logs:/app/logs
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8899/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
        reservations:
          cpus: '2'
          memory: 4G
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "10"

  # Optional: Reverse proxy
  nginx:
    image: nginx:alpine
    container_name: cortex-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - hf-voice
    restart: unless-stopped
```

### Step 3: Deploy

```bash
# Build and start services
docker-compose up -d

# View logs
docker-compose logs -f hf-voice

# Check health
curl http://localhost:8899/health

# Stop services
docker-compose down
```

---

## Production Configuration

### HF Service Configuration

```yaml
# config.production.yaml
service:
  host: "0.0.0.0"
  port: 8899
  workers: 4  # CPU cores
  timeout: 30
  max_concurrent_requests: 100

  cors_origins:
    - "https://cortexavatar.com"
    - "https://app.cortexavatar.com"

models:
  cache_dir: "/app/models"

  vad:
    repo: "snakers4/silero-vad"
    model: "silero_vad.onnx"

  stt:
    repo: "openai/whisper-large-v3-turbo"
    language: "en"
    device: "cpu"  # or "cuda" for GPU

  tts:
    repo: "myshell-ai/MeloTTS-English"
    speaker: "EN-US"
    speed: 1.0

performance:
  batch_size: 1
  max_concurrent_requests: 100
  request_queue_size: 200

  # Memory optimization
  model_caching: true
  inference_threads: 4

logging:
  level: "INFO"  # DEBUG, INFO, WARNING, ERROR
  format: "json"
  output: "/app/logs/service.log"
  rotation: "daily"
  retention_days: 30

monitoring:
  enabled: true
  prometheus_port: 9090
  metrics_endpoint: "/metrics"

security:
  api_key_required: true
  rate_limit:
    enabled: true
    max_requests_per_minute: 60
  allowed_ips:
    - "10.0.0.0/8"
    - "172.16.0.0/12"
```

### Environment Variables

```bash
# .env.production
LOG_LEVEL=INFO
MAX_WORKERS=4
TIMEOUT=30
API_KEY=your-secret-api-key-here
PROMETHEUS_ENABLED=true
CORS_ORIGINS=https://cortexavatar.com,https://app.cortexavatar.com
```

### Nginx Reverse Proxy

```nginx
# nginx.conf
upstream hf_voice {
    server hf-voice:8899;
    keepalive 32;
}

server {
    listen 80;
    server_name voice.cortexavatar.com;

    # Redirect HTTP to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name voice.cortexavatar.com;

    # SSL Configuration
    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=voice_limit:10m rate=10r/s;
    limit_req zone=voice_limit burst=20 nodelay;

    # Client body size
    client_max_body_size 10M;

    # Timeouts
    proxy_connect_timeout 30s;
    proxy_send_timeout 30s;
    proxy_read_timeout 30s;

    # Proxy settings
    location / {
        proxy_pass http://hf_voice;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Buffering
        proxy_buffering off;
        proxy_request_buffering off;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://hf_voice/health;
        access_log off;
    }

    # Metrics endpoint (restrict access)
    location /metrics {
        proxy_pass http://hf_voice:9090/metrics;
        allow 10.0.0.0/8;
        deny all;
    }
}
```

---

## Monitoring & Logging

### Prometheus Metrics

```python
# service.py - Metrics instrumentation
from prometheus_client import Counter, Histogram, Gauge
from prometheus_client import start_http_server

# Request metrics
request_count = Counter('hf_voice_requests_total', 'Total requests', ['endpoint', 'status'])
request_latency = Histogram('hf_voice_request_duration_seconds', 'Request latency', ['endpoint'])

# Model metrics
model_inference_time = Histogram('hf_voice_model_inference_seconds', 'Model inference time', ['model'])
active_requests = Gauge('hf_voice_active_requests', 'Active requests')

# Memory metrics
memory_usage = Gauge('hf_voice_memory_bytes', 'Memory usage')

# Start Prometheus server
start_http_server(9090)
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "HF Voice Pipeline",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [{
          "expr": "rate(hf_voice_requests_total[5m])"
        }]
      },
      {
        "title": "P95 Latency",
        "targets": [{
          "expr": "histogram_quantile(0.95, hf_voice_request_duration_seconds)"
        }]
      },
      {
        "title": "Memory Usage",
        "targets": [{
          "expr": "hf_voice_memory_bytes"
        }]
      },
      {
        "title": "Active Requests",
        "targets": [{
          "expr": "hf_voice_active_requests"
        }]
      }
    ]
  }
}
```

### Logging Configuration

```python
# logging_config.py
import logging
from logging.handlers import RotatingFileHandler
import json

class JSONFormatter(logging.Formatter):
    def format(self, record):
        log_data = {
            "timestamp": self.formatTime(record),
            "level": record.levelname,
            "message": record.getMessage(),
            "module": record.module,
            "function": record.funcName,
        }
        if record.exc_info:
            log_data["exception"] = self.formatException(record.exc_info)
        return json.dumps(log_data)

# Configure logging
handler = RotatingFileHandler(
    'logs/service.log',
    maxBytes=100*1024*1024,  # 100MB
    backupCount=10
)
handler.setFormatter(JSONFormatter())

logger = logging.getLogger('hf_voice')
logger.addHandler(handler)
logger.setLevel(logging.INFO)
```

### Log Aggregation (ELK Stack)

```yaml
# docker-compose.elk.yml
version: '3.8'

services:
  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.11.0
    environment:
      - discovery.type=single-node
      - "ES_JAVA_OPTS=-Xms512m -Xmx512m"
    volumes:
      - esdata:/usr/share/elasticsearch/data
    ports:
      - "9200:9200"

  logstash:
    image: docker.elastic.co/logstash/logstash:8.11.0
    volumes:
      - ./logstash.conf:/usr/share/logstash/pipeline/logstash.conf
    depends_on:
      - elasticsearch

  kibana:
    image: docker.elastic.co/kibana/kibana:8.11.0
    ports:
      - "5601:5601"
    depends_on:
      - elasticsearch

volumes:
  esdata:
```

---

## Scaling Strategies

### Horizontal Scaling

```yaml
# docker-compose.scale.yml
version: '3.8'

services:
  hf-voice:
    build: .
    deploy:
      replicas: 3  # Run 3 instances
      resources:
        limits:
          cpus: '2'
          memory: 6G
    environment:
      - WORKER_ID=${WORKER_ID}

  load-balancer:
    image: nginx:alpine
    ports:
      - "8899:80"
    volumes:
      - ./nginx-lb.conf:/etc/nginx/nginx.conf
    depends_on:
      - hf-voice
```

```nginx
# nginx-lb.conf
upstream hf_voice_cluster {
    least_conn;  # Load balancing algorithm
    server hf-voice-1:8899;
    server hf-voice-2:8899;
    server hf-voice-3:8899;
}

server {
    listen 80;

    location / {
        proxy_pass http://hf_voice_cluster;
        proxy_next_upstream error timeout invalid_header http_500;
        proxy_connect_timeout 2s;
    }
}
```

### Kubernetes Deployment

```yaml
# k8s-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hf-voice
  labels:
    app: hf-voice
spec:
  replicas: 3
  selector:
    matchLabels:
      app: hf-voice
  template:
    metadata:
      labels:
        app: hf-voice
    spec:
      containers:
      - name: hf-voice
        image: cortexavatar/hf-voice:2.4.0
        ports:
        - containerPort: 8899
        env:
        - name: LOG_LEVEL
          value: "INFO"
        - name: MAX_WORKERS
          value: "4"
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "8Gi"
            cpu: "4"
        livenessProbe:
          httpGet:
            path: /health
            port: 8899
          initialDelaySeconds: 60
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8899
          initialDelaySeconds: 30
          periodSeconds: 10

---
apiVersion: v1
kind: Service
metadata:
  name: hf-voice-service
spec:
  selector:
    app: hf-voice
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8899
  type: LoadBalancer

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: hf-voice-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: hf-voice
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Auto-Scaling Configuration

```bash
# Deploy with auto-scaling
kubectl apply -f k8s-deployment.yaml

# Monitor scaling
kubectl get hpa hf-voice-hpa --watch

# Test scaling
kubectl run -it --rm load-generator --image=busybox /bin/sh
while true; do wget -q -O- http://hf-voice-service/health; done
```

---

## Security Hardening

### 1. API Key Authentication

```python
# middleware.py
from fastapi import Security, HTTPException
from fastapi.security import APIKeyHeader

api_key_header = APIKeyHeader(name="X-API-Key")

async def verify_api_key(api_key: str = Security(api_key_header)):
    if api_key != os.getenv("API_KEY"):
        raise HTTPException(status_code=403, detail="Invalid API key")
    return api_key

# Apply to endpoints
@app.post("/stt", dependencies=[Security(verify_api_key)])
async def transcribe(audio: UploadFile):
    # ...
```

### 2. Rate Limiting

```python
# rate_limit.py
from slowapi import Limiter
from slowapi.util import get_remote_address

limiter = Limiter(key_func=get_remote_address)

@app.post("/stt")
@limiter.limit("10/minute")
async def transcribe(request: Request, audio: UploadFile):
    # ...
```

### 3. Input Validation

```python
# validation.py
from pydantic import BaseModel, validator

class TTSRequest(BaseModel):
    text: str
    language: str = "EN"
    speed: float = 1.0

    @validator('text')
    def validate_text(cls, v):
        if len(v) > 5000:
            raise ValueError('Text too long (max 5000 chars)')
        if not v.strip():
            raise ValueError('Text cannot be empty')
        return v

    @validator('speed')
    def validate_speed(cls, v):
        if not 0.5 <= v <= 2.0:
            raise ValueError('Speed must be between 0.5 and 2.0')
        return v
```

### 4. HTTPS Configuration

```bash
# Generate SSL certificate with Let's Encrypt
certbot certonly --standalone -d voice.cortexavatar.com

# Configure nginx
server {
    listen 443 ssl http2;
    ssl_certificate /etc/letsencrypt/live/voice.cortexavatar.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/voice.cortexavatar.com/privkey.pem;

    # Strong SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512;
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
}
```

---

## Troubleshooting Production Issues

### Issue 1: High Memory Usage

**Symptoms:**
- Service using >8GB RAM
- OOM kills in logs

**Diagnosis:**
```bash
# Check memory usage
docker stats cortex-hf-voice

# Check logs
docker logs cortex-hf-voice | grep -i "memory"

# Profile memory
python -m memory_profiler service.py
```

**Solutions:**
1. Reduce concurrent requests:
   ```yaml
   max_concurrent_requests: 50  # Reduce from 100
   ```

2. Enable model offloading:
   ```python
   # Load models on-demand
   model_cache = {}

   def get_model(model_name):
       if model_name not in model_cache:
           model_cache[model_name] = load_model(model_name)
       return model_cache[model_name]
   ```

3. Increase memory limit:
   ```yaml
   deploy:
     resources:
       limits:
         memory: 12G  # Increase from 8G
   ```

### Issue 2: High Latency

**Symptoms:**
- P95 latency >2s
- Slow responses

**Diagnosis:**
```bash
# Check service metrics
curl http://localhost:9090/metrics | grep latency

# Monitor request queue
curl http://localhost:8899/health | jq '.queue_size'

# Check CPU usage
top -pid $(pgrep -f "python service.py")
```

**Solutions:**
1. Scale horizontally:
   ```bash
   docker-compose up -d --scale hf-voice=3
   ```

2. Optimize inference:
   ```python
   # Use quantized models
   model = AutoModel.from_pretrained(
       "openai/whisper-large-v3-turbo",
       torch_dtype=torch.float16  # Use FP16
   )
   ```

3. Add caching:
   ```python
   from functools import lru_cache

   @lru_cache(maxsize=100)
   def transcribe_cached(audio_hash):
       return transcribe(audio)
   ```

### Issue 3: Service Crashes

**Symptoms:**
- Container restarts frequently
- Health checks failing

**Diagnosis:**
```bash
# Check logs
docker logs cortex-hf-voice --tail 100

# Check crash dump
docker inspect cortex-hf-voice | jq '.[0].State'

# Test manually
python service.py --debug
```

**Solutions:**
1. Increase timeout:
   ```yaml
   timeout: 60  # Increase from 30
   ```

2. Add error recovery:
   ```python
   try:
       result = process_audio(audio)
   except Exception as e:
       logger.error(f"Processing failed: {e}")
       # Return partial result or error response
       return {"error": str(e)}
   ```

3. Enable auto-restart:
   ```yaml
   restart: always  # Change from unless-stopped
   ```

### Issue 4: Connection Refused

**Symptoms:**
- Client can't connect to service
- Connection timeout errors

**Diagnosis:**
```bash
# Check if service is running
docker ps | grep hf-voice

# Check port binding
netstat -tulpn | grep 8899

# Test connection
curl http://localhost:8899/health
telnet localhost 8899
```

**Solutions:**
1. Check firewall:
   ```bash
   sudo ufw allow 8899
   ```

2. Verify network mode:
   ```yaml
   network_mode: "host"  # Or bridge
   ```

3. Check bind address:
   ```python
   # service.py
   uvicorn.run(app, host="0.0.0.0", port=8899)  # Not 127.0.0.1
   ```

---

## Backup & Recovery

### Model Backup

```bash
# Backup models
tar -czf models-backup-$(date +%Y%m%d).tar.gz /app/models

# Upload to S3
aws s3 cp models-backup-*.tar.gz s3://cortex-backups/models/

# Restore models
aws s3 cp s3://cortex-backups/models/latest.tar.gz .
tar -xzf latest.tar.gz -C /app/
```

### Configuration Backup

```bash
# Backup configuration
git add config.production.yaml
git commit -m "prod: update HF service config"
git push origin production

# Rollback configuration
git revert HEAD
git push origin production
```

### Disaster Recovery

```bash
# Full service recovery
docker pull cortexavatar/hf-voice:2.4.0
docker-compose down
docker-compose up -d

# Verify recovery
curl http://localhost:8899/health
```

---

## Performance Benchmarks

### Expected Performance

| Metric | Target | Acceptable | Critical |
|--------|--------|-----------|----------|
| **E2E Latency (P95)** | <2s | <3s | >5s |
| **STT Latency (P95)** | <500ms | <1s | >2s |
| **TTS Latency (P95)** | <700ms | <1.5s | >3s |
| **Memory Usage** | <6GB | <8GB | >10GB |
| **Success Rate** | >99% | >95% | <90% |
| **Concurrent Requests** | 100 | 50 | <20 |

### Load Testing

```bash
# Install Apache Bench
brew install apache-bench

# Test health endpoint
ab -n 1000 -c 10 http://localhost:8899/health

# Test STT endpoint
ab -n 100 -c 5 -p audio.wav http://localhost:8899/stt

# View results
# Requests per second: 45.23
# Time per request (mean): 110ms
# 95th percentile: 250ms
```

---

## Maintenance

### Regular Tasks

**Daily:**
- Monitor error logs
- Check disk space
- Verify health checks

**Weekly:**
- Review performance metrics
- Check for security updates
- Rotate logs

**Monthly:**
- Update models
- Review capacity planning
- Backup configurations

### Update Procedure

```bash
# 1. Backup current state
docker-compose exec hf-voice tar -czf /backups/state-$(date +%Y%m%d).tar.gz /app

# 2. Pull new image
docker pull cortexavatar/hf-voice:2.5.0

# 3. Update docker-compose.yml
sed -i 's/2.4.0/2.5.0/g' docker-compose.yml

# 4. Rolling update
docker-compose up -d --no-deps --build hf-voice

# 5. Verify
curl http://localhost:8899/health

# 6. Rollback if needed
docker-compose down
docker pull cortexavatar/hf-voice:2.4.0
docker-compose up -d
```

---

## Resources

- **User Guide:** [HF_VOICE_USER_GUIDE.md](HF_VOICE_USER_GUIDE.md)
- **Developer Guide:** [HF_VOICE_DEV_GUIDE.md](HF_VOICE_DEV_GUIDE.md)
- **GitHub:** https://github.com/normanking/cortex-avatar
- **Docker Hub:** https://hub.docker.com/r/cortexavatar/hf-voice

---

**Need Production Support?**
- Email: ops@cortexavatar.com
- Slack: #hf-voice-ops
- On-call: +1-555-CORTEX-OPS
