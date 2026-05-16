# Sushiro Queue Collector

Cloudflare Workers + D1 collector for optional queue-stat contribution.

The client uploads only aggregated buckets. The Worker rejects common sensitive
fields such as authorization tokens, phone numbers, WeChat IDs, raw ticket
numbers, precise timestamps, and single-session traces.

## Deploy

Cloudflare D1 uses a Worker binding declared in `wrangler.toml`. The Worker
queries D1 through `env.DB.prepare(...).bind(...).run()` / `batch()`.

```bash
cd collector
npm install
npx wrangler login
npx wrangler d1 create sushiro-queue-collector
```

Copy the generated `database_id` into `wrangler.toml`, then run:

```bash
npx wrangler d1 migrations apply sushiro-queue-collector --remote
npx wrangler deploy
```

Set the client collector URL to:

```text
https://<your-worker>.<your-subdomain>.workers.dev/v1/submit
```

or bind a custom domain such as:

```text
https://queue.sushiro-overdose.com/v1/submit
```

## API

```http
POST /v1/submit
GET  /v1/stats?store_id=<id>
GET  /health
```

`POST /v1/submit` accepts schema version 1 payloads emitted by the desktop
client's contribution preview page. It does not accept raw local JSONL records.

## Sources

Implementation follows the current Cloudflare D1 Worker binding pattern and
Wrangler D1 database binding configuration documented by Cloudflare:

- https://developers.cloudflare.com/d1/worker-api/
- https://developers.cloudflare.com/workers/wrangler/configuration/
