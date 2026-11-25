import type { ReactElement } from "react";

export interface HUDWindow {
  group?: string;
  kind: string;
  label?: string;
  remaining_percent?: number | null;
  reset_description?: string;
  status: string;
}
export interface HUDProvider {
  provider: string;
  profile: string;
  display_name?: string;
  health: string;
  availability: string;
  capacity_state: string;
  freshness: string;
  source?: string;
  windows?: HUDWindow[];
}
export interface HUDCostSummary {
  evidence: string;
  known_microusd?: number | null;
  estimated_microusd?: number | null;
  unknown_observation_count?: number;
  budget_level?: string;
}
export interface HUDState {
  providers?: HUDProvider[];
  cost?: HUDCostSummary | null;
  primary?: { kind: string; id?: string };
}
export interface Labels {
  control: string;
  hud: string;
  unknown: string;
  noAccounts: string;
  remaining: string;
  knownCost: string;
  estimatedCost: string;
  unknownCost: string;
  cost: string;
  noActivity: string;
  budget: string;
}
export interface DisplayProps {
  locale?: string;
  labels?: Partial<Labels>;
}
export function ProviderCard(props: DisplayProps & { provider: HUDProvider }): ReactElement;
export function CostPanel(props: DisplayProps & { summary?: HUDCostSummary | null }): ReactElement;
export function AIControl(props: DisplayProps & { state: HUDState }): ReactElement;
export function HUD(props: DisplayProps & { state: HUDState }): ReactElement;
