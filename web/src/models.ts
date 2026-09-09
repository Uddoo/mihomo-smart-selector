export interface Health {
  status: 'ok' | 'degraded'
  mihomo_connected: boolean
  mihomo_version?: string
  error?: string
}

export interface RuntimeSettings {
 max_active_scans: number
 retention: {scan_days: number; max_scans: number; audit_days: number; max_audit: number}
	refine_top_k: number
	result_max_age_seconds: number
  revision: number
  concurrency: number
  batch_size: number
  max_candidates: number
  samples: number
  timeout_ms: number
  min_success_rate: number
  median_target_ms: number
  p95_target_ms: number
  jitter_target_ms: number
  default_profile: string
  egress: {enabled: boolean; selector_group: string; proxy_url: string; trace_url: string}
  strict: {enabled: boolean; selector_group: string; proxy_url: string; max_candidates: number}
}

export interface Group {
  name: string
  type: string
  now?: string
  all?: string[]
}

export interface ServiceBinding {
  group: string
  profile_id: string
  status?: 'valid' | 'group_missing' | 'profile_missing'
}

export interface ServiceCatalog {
  profiles: ProbeProfileSummary[]
  bindings: ServiceBinding[]
  suggestions: Record<string, string>
  default_profile_id: string
}

export interface Provider {
  name: string
  proxies?: Array<{ name: string; type?: string; 'provider-name'?: string }>
}

export interface Region {
  code: string
  name: string
  emoji?: string
}

export interface ProbeSample {
  probe: string
  delay_ms?: number
  error?: string
}

export interface StrictCheck {
  probe: string
  expected_status: string
  observed_status?: number
  status: string
  body_matched?: boolean
  error?: string
}

export interface ScoreBreakdown {
  reliability: number
  p50: number
  p95: number
  jitter: number
  region: number
  total: number
}

export interface ProbeProfileSummary {
	require_strict?: boolean
	require_region?: boolean
  id: string
  label: string
  description: string
  probe_count: number
  strict_probe_count: number
  strict_verification_available: boolean
  requires_configuration: boolean
  setup_hint?: string
  expected_regions?: string[]
  transport_scope: string
  targets?: ProbeTargetSummary[]
}

export interface ProbeTargetSummary {
  name: string
  address?: string
  expected_status: string
  kind: 'reachability' | 'strict'
  address_visible: boolean
}

export interface NodeResult {
	screening_samples?: number
	refinement_samples?: number
	stage?: 'screened' | 'refined'
	measured_at?: string
	expires_at?: string
	selection_reason?: string
  rank: number
  name: string
  provider?: string
  inferred_region?: string
  region_source?: string
  verified_region?: string
  egress_error?: string
  samples: ProbeSample[]
  strict_checks?: StrictCheck[]
  success_rate: number
  p50_ms?: number
  p95_ms?: number
  jitter_ms?: number
  score: number
  score_breakdown: ScoreBreakdown
  reachability_status: string
  strict_verification_status: string
  restriction_status: string
  region_verification_status: string
  transport_status: string
}

export interface Scan {
  id: string
  status: 'running' | 'complete' | 'failed' | 'cancelled' | 'interrupted'
  request: {
	nodes?: string[]
    target_group: string
    profile_id?: string
    regions: string[]
    providers: string[]
    mode: 'stable' | 'quick'
  }
  profile: ProbeProfileSummary
  progress: ScanProgress
  started_at: string
  completed_at?: string
  error?: string
  results: NodeResult[]
}

export interface ScanProgress {
 stage?: string
  completed: number
  total: number
  current_batch: number
  total_batches: number
  batch_completed: number
  batch_total: number
  succeeded: number
  failed: number
  elapsed_seconds: number
  estimated_remaining_seconds: number
  stop_after_current_batch: boolean
}

export interface ScanPreview {
 refine_candidates: number
  candidate_count: number
  batch_size: number
  batch_count: number
  samples_per_node: number
  probe_requests: number
  profile: ProbeProfileSummary
  ready: boolean
  reason?: string
}

export interface NodeSummary {
  name: string
  provider?: string
  protocol?: string
  inferred_region?: string
  region_source: string
  entry_kind?: 'proxy' | 'builtin' | 'subscription-info' | 'dynamic'
  region_reason?: 'transit' | 'conflicting-cues'
  region_candidates?: string[]
  region_evidence?: string[]
}

export interface SwitchEvent {
 status: string
 request_id?: string
 audit_persisted: boolean
  id: number
  scan_id: string
  group: string
  previous?: string
  selected: string
  reason: string
  created_at: string
}
