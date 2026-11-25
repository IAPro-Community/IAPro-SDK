import test from "node:test";
import assert from "node:assert/strict";
import { createElement as h } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { AIControl, HUD, ProviderCard } from "./index.js";

const provider = {
  provider: "codex", profile: "a", health: "HEALTHY", availability: "AVAILABLE",
  capacity_state: "HEALTHY", freshness: "LIVE",
  windows: [
    { kind: "5h", status: "LIVE", remaining_percent: 0 },
    { kind: "weekly", status: "UNKNOWN", remaining_percent: null },
  ],
};

test("measured zero differs from unknown and stale", () => {
  const html = renderToStaticMarkup(h(ProviderCard, { provider, locale: "en-US" }));
  assert.match(html, /0%/);
  assert.match(html, /Unknown/);
  assert.equal((html.match(/<meter/g) || []).length, 1);
  const stale = renderToStaticMarkup(h(ProviderCard, { provider: { ...provider, freshness: "STALE" } }));
  assert.doesNotMatch(stale, /<meter/);
});

test("cost evidence and backend focus are not fabricated", () => {
  const state = { providers: [], primary: { kind: "ALERT", id: "critical" }, cost: { evidence: "ATTRIBUTION_ONLY", known_microusd: 0 } };
  const html = renderToStaticMarkup(h(HUD, { state, labels: { unknown: "Sem evidência" } }));
  assert.match(html, /ALERT · critical/);
  assert.match(html, /Sem evidência/);
  assert.doesNotMatch(html, /\$0/);
  assert.match(renderToStaticMarkup(h(AIControl, { state: {} })), /No provider accounts/);
});

test("partial and estimated zero do not claim measured zero", () => {
  const state = { providers: [], cost: { evidence: "ESTIMATED", known_microusd: 0, estimated_microusd: 1000000 } };
  const html = renderToStaticMarkup(h(AIControl, { state, locale: "en-US" }));
  assert.match(html, /Measured cost<\/dt><dd>Unknown/);
  assert.match(html, /Estimated cost<\/dt><dd>\$1.00/);
});
