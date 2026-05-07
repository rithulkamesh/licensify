# Deployment

## Local (recommended for development)

Use Docker Compose from repo root:

```bash
docker compose up -d --build
curl -sS http://localhost:8080/v1/health
```

### Environment variables

- `DATABASE_URL`: Postgres DSN used by the server
- `LICENSIFY_API_KEY`: API key required for non-public/admin routes

If you’re using `docker compose`, these are usually provided via the compose file and/or a `.env`.

## Production notes

### Networking and TLS

- Terminate TLS in front of the server (reverse proxy / ingress).
- Restrict access to admin routes (private network, auth gateway, or both).

### Postgres

- Run Postgres with backups and monitoring.
- Treat Postgres as the only durable state; keep server instances stateless.

### Secrets and logging

- Rotate `LICENSIFY_API_KEY` and keep it out of logs.
- Avoid logging full license keys or machine identifiers at info level.

### Health checks

- Use `GET /v1/health` for liveness/readiness.
- Consider surfacing build/version info from the health response in your deployment dashboard.
