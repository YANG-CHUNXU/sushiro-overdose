const MAX_BODY_BYTES = 256 * 1024;
const MAX_STATS = 500;
const MIN_SAMPLES = 5;
const SENSITIVE_KEYS = [
  "authorization",
  "query_authorization",
  "reservation_authorization",
  "phone",
  "phone_number",
  "wechat",
  "wechat_id",
  "ticket_no",
  "display_called_no",
  "called_no_when_user_called",
  "taken_at",
  "checked_in_at",
  "called_for_user_at",
  "single_session_trace"
];

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (request.method === "OPTIONS") return cors(new Response(null, { status: 204 }));
    if (url.pathname === "/health") return json({ ok: true });
    if (url.pathname === "/v1/submit" && request.method === "POST") return submit(request, env);
    if (url.pathname === "/v1/stats" && request.method === "GET") return stats(url, env);
    return json({ error: "not found" }, 404);
  }
};

async function submit(request, env) {
  const contentLength = Number(request.headers.get("content-length") || 0);
  if (contentLength > MAX_BODY_BYTES) return json({ error: "payload too large" }, 413);

  const payload = await request.json().catch(() => null);
  if (!payload || typeof payload !== "object") return json({ error: "invalid json" }, 400);
  const sensitive = findSensitiveKeys(payload);
  if (sensitive.length) return json({ error: "sensitive fields are not accepted", fields: sensitive }, 400);
  const err = validatePayload(payload);
  if (err) return json({ error: err }, 400);

  const statements = payload.stats.map((s) =>
    env.DB.prepare(`
      INSERT INTO queue_stats (
        schema_version, client_version, install_id_hash,
        store_id, weekday, time_bucket, table_type, party_size_bucket,
        samples, wait_p50_minutes, wait_p80_minutes, checkin_to_call_p50_minutes, missed_rate
      ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).bind(
      payload.schema_version,
      String(payload.client_version || "unknown").slice(0, 32),
      payload.install_id_hash,
      s.store_id,
      s.weekday,
      s.time_bucket,
      s.table_type,
      s.party_size_bucket,
      s.samples,
      nullableNumber(s.wait_p50_minutes),
      nullableNumber(s.wait_p80_minutes),
      nullableNumber(s.checkin_to_call_p50_minutes),
      s.missed_rate
    )
  );
  await env.DB.batch(statements);
  return json({ ok: true, accepted: payload.stats.length });
}

async function stats(url, env) {
  const storeID = (url.searchParams.get("store_id") || "").trim();
  if (!/^[A-Za-z0-9_-]{1,64}$/.test(storeID)) return json({ error: "store_id required" }, 400);
  const result = await env.DB.prepare(`
    SELECT
      store_id,
      weekday,
      time_bucket,
      table_type,
      party_size_bucket,
      SUM(samples) AS samples,
      AVG(wait_p50_minutes) AS wait_p50_minutes,
      AVG(wait_p80_minutes) AS wait_p80_minutes,
      AVG(checkin_to_call_p50_minutes) AS checkin_to_call_p50_minutes,
      AVG(missed_rate) AS missed_rate,
      MAX(received_at) AS latest_received_at
    FROM queue_stats
    WHERE store_id = ?
      AND received_at >= datetime('now', '-90 days')
    GROUP BY store_id, weekday, time_bucket, table_type, party_size_bucket
    HAVING SUM(samples) >= ?
    ORDER BY weekday, time_bucket, table_type, party_size_bucket
    LIMIT 500
  `).bind(storeID, MIN_SAMPLES).all();
  return json({ ok: true, store_id: storeID, stats: result.results || [] });
}

function validatePayload(payload) {
  if (payload.schema_version !== 1) return "unsupported schema_version";
  if (!/^[a-f0-9]{64}$/.test(String(payload.install_id_hash || ""))) return "invalid install_id_hash";
  if (!Array.isArray(payload.stats)) return "stats must be an array";
  if (payload.stats.length === 0) return "stats is empty";
  if (payload.stats.length > MAX_STATS) return "too many stats";
  for (const s of payload.stats) {
    if (!/^[A-Za-z0-9_-]{1,64}$/.test(String(s.store_id || ""))) return "invalid store_id";
    if (!Number.isInteger(s.weekday) || s.weekday < 1 || s.weekday > 7) return "invalid weekday";
    if (!/^\d{2}:\d{2}$/.test(String(s.time_bucket || ""))) return "invalid time_bucket";
    if (!["T", "C", "unknown"].includes(s.table_type)) return "invalid table_type";
    if (!["1-2", "3-4", "5+", "unknown"].includes(s.party_size_bucket)) return "invalid party_size_bucket";
    if (!Number.isInteger(s.samples) || s.samples < MIN_SAMPLES || s.samples > 100000) return "invalid samples";
    if (!optionalNumber(s.wait_p50_minutes)) return "invalid wait_p50_minutes";
    if (!optionalNumber(s.wait_p80_minutes)) return "invalid wait_p80_minutes";
    if (!optionalNumber(s.checkin_to_call_p50_minutes)) return "invalid checkin_to_call_p50_minutes";
    if (typeof s.missed_rate !== "number" || s.missed_rate < 0 || s.missed_rate > 1) return "invalid missed_rate";
  }
  return "";
}

function findSensitiveKeys(value, path = "") {
  if (!value || typeof value !== "object") return [];
  const out = [];
  for (const [key, child] of Object.entries(value)) {
    const lower = key.toLowerCase();
    if (SENSITIVE_KEYS.some((k) => lower.includes(k))) out.push(path ? `${path}.${key}` : key);
    if (child && typeof child === "object") out.push(...findSensitiveKeys(child, path ? `${path}.${key}` : key));
  }
  return [...new Set(out)].slice(0, 20);
}

function optionalNumber(v) {
  return v == null || (typeof v === "number" && Number.isFinite(v) && v >= 0 && v <= 24 * 60);
}

function nullableNumber(v) {
  return optionalNumber(v) && v != null ? v : null;
}

function json(value, status = 200) {
  return cors(new Response(JSON.stringify(value), {
    status,
    headers: { "content-type": "application/json; charset=utf-8" }
  }));
}

function cors(response) {
  const headers = new Headers(response.headers);
  headers.set("access-control-allow-origin", "*");
  headers.set("access-control-allow-methods", "GET,POST,OPTIONS");
  headers.set("access-control-allow-headers", "content-type");
  return new Response(response.body, { status: response.status, headers });
}
