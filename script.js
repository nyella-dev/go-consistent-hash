import http from "k6/http";
import { check, sleep } from "k6";
import { Counter } from "k6/metrics";

export const options = {
  vus: 20,
  duration: "10s",
};

const BASE_URL = "http://localhost:8080";

// declare all counters in init context
const hits = {
  "server1": new Counter("hits_server1"),
  "server2": new Counter("hits_server2"),
  "server3": new Counter("hits_server3"),
};

export default function () {
  const userID = Math.floor(Math.random() * 100_000).toString();
  const res = http.get(`${BASE_URL}/user/${userID}`);

  check(res, {
    "status 200":       (r) => r.status === 200,
    "has server field": (r) => {
      try { return JSON.parse(r.body).server !== undefined; }
      catch { return false; }
    },
  });

  if (res.status === 200) {
    try {
      const body    = JSON.parse(res.body);
      const server  = body.server.replace(/-/g, ""); // "server-3" -> "server3"
      if (hits[server]) {
        hits[server].add(1);
      } else {
        console.warn(`unknown server: ${body.server}`);
      }
    } catch (e) {
      console.error(`parse error: ${res.body} — ${e}`);
    }
  }

  sleep(0.05);
}

export function handleSummary(data) {
  const lines = ["\n=== Consistent Hash Distribution ==="];

  const entries = Object.keys(data.metrics)
    .filter((k) => k.startsWith("hits_"))
    .map((k) => ({
      server: k.replace("hits_", ""),
      count:  data.metrics[k].values.count,
    }))
    .sort((a, b) => b.count - a.count);

  const total = entries.reduce((s, e) => s + e.count, 0);
  const pad   = Math.max(...entries.map((e) => e.server.length));

  for (const { server, count } of entries) {
    const pct = total ? ((count / total) * 100).toFixed(1) : "0.0";
    const bar = "█".repeat(Math.round((count / total) * 30));
    lines.push(`  ${server.padEnd(pad)}  ${String(count).padStart(6)}  (${pct.padStart(5)}%)  ${bar}`);
  }

  lines.push(`  ${"TOTAL".padEnd(pad)}  ${String(total).padStart(6)}`);
  lines.push("=====================================\n");

  return { stdout: lines.join("\n") };
}