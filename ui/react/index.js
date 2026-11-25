// SPDX-License-Identifier: MIT
import { createElement as h } from "react";

const defaults = Object.freeze({
  control: "AI Control",
  hud: "Activity",
  unknown: "Unknown",
  noAccounts: "No provider accounts",
  remaining: "Remaining",
  knownCost: "Measured cost",
  estimatedCost: "Estimated cost",
  unknownCost: "Unpriced observations",
  cost: "Cost",
  noActivity: "No active work",
  budget: "Budget",
});
const labelsFor = (labels) => ({ ...defaults, ...labels });
const observed = (w) =>
  ["LIVE", "CACHED", "RATE_LIMITED"].includes(w.status) &&
  typeof w.remaining_percent === "number" &&
  Number.isFinite(w.remaining_percent) &&
  w.remaining_percent >= 0 && w.remaining_percent <= 100;
const fresh = (provider) => ["LIVE", "CACHED", "FRESH"].includes(provider.freshness);

function quotaWindow(w, i, provider, labels, locale) {
  const title = w.label || [w.group, w.kind].filter(Boolean).join(" / ");
  const known = fresh(provider) && observed(w);
  const value = known
    ? new Intl.NumberFormat(locale, { maximumFractionDigits: 1, style: "percent" }).format(w.remaining_percent / 100)
    : labels.unknown;
  return h("li", { key: i, className: "iapro-quota-window" },
    h("span", null, title),
    h("span", null, value),
    known ? h("meter", { min: 0, max: 100, value: w.remaining_percent, "aria-label": title + " " + labels.remaining }) : null,
    w.reset_description ? h("small", null, w.reset_description) : null,
  );
}

export function ProviderCard({ provider, labels, locale }) {
  const l = labelsFor(labels);
  return h("article", { className: "iapro-provider", "aria-label": provider.display_name || provider.provider },
    h("h3", null, provider.display_name || provider.provider, " / ", provider.profile),
    h("p", null, [provider.health, provider.availability, provider.capacity_state].filter(Boolean).join(" · ")),
    h("small", null, [provider.source, provider.freshness].filter(Boolean).join(" · ")),
    h("ul", null, (provider.windows || []).map((w, i) => quotaWindow(w, i, provider, l, locale))),
  );
}

function money(value, unknown, locale) {
  if (!Number.isSafeInteger(value) || value < 0) return unknown;
  return new Intl.NumberFormat(locale, { style: "currency", currency: "USD", maximumFractionDigits: 6 }).format(value / 1_000_000);
}

export function CostPanel({ summary, labels, locale }) {
  const l = labelsFor(labels);
  return h("section", { className: "iapro-cost", "aria-label": l.cost },
    h("h3", null, l.cost),
    summary == null || ["EMPTY", "ATTRIBUTION_ONLY", "UNKNOWN"].includes(summary.evidence)
      ? h("p", null, l.unknown)
      : h("dl", null,
          h("dt", null, l.knownCost), h("dd", null, summary.evidence === "MEASURED" || summary.known_observation_count > 0 || summary.known_microusd > 0 ? money(summary.known_microusd, l.unknown, locale) : l.unknown),
          summary.estimated_microusd == null ? null : h("dt", null, l.estimatedCost),
          summary.estimated_microusd == null ? null : h("dd", null, money(summary.estimated_microusd, l.unknown, locale)),
          h("dt", null, l.unknownCost), h("dd", null, summary.unknown_observation_count ?? l.unknown),
          h("dt", null, l.budget), h("dd", null, summary.budget_level || l.unknown)),
  );
}

export function AIControl({ state, labels, locale }) {
  const l = labelsFor(labels);
  const accounts = state.providers || [];
  return h("section", { className: "iapro-control", "aria-label": l.control },
    h("h2", null, l.control),
    accounts.length === 0 ? h("p", null, l.noAccounts) : accounts.map((provider) =>
      h(ProviderCard, { key: JSON.stringify([provider.provider, provider.profile]), provider, labels: l, locale })),
    h(CostPanel, { summary: state.cost, labels: l, locale }),
  );
}

export function HUD({ state, labels, locale }) {
  const l = labelsFor(labels);
  const focus = state.primary || { kind: "EMPTY" };
  // Focus is chosen by the backend; never recalculate quota or routing here.
  return h("section", { className: "iapro-hud", "aria-label": l.hud },
    h("h2", null, l.hud),
    h("p", { role: "status" }, focus.kind === "EMPTY" ? l.noActivity : [focus.kind, focus.id].filter(Boolean).join(" · ")),
    h(AIControl, { state, labels: l, locale }),
  );
}
