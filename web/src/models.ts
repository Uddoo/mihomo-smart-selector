export interface Health {
  status: 'ok' | 'degraded'
  mihomo_connected: boolean
  mihomo_version?: string
  error?: string
}

export interface Group {
  name: string
  type: string
  now?: string
  all?: string[]
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

export interface NodeResult {
  rank: number
  name: string
  provider?: string
  inferred_region?: string
  region_source?: string
  verified_region?: string
  egress_error?: string
  samples: ProbeSample[]
  success_rate: number
  p50_ms?: number
  p95_ms?: number
  jitter_ms?: number
  score: number
}

export interface Scan {
  id: string
  status: 'running' | 'complete' | 'failed' | 'cancelled'
  request: {
    target_group: string
    regions: string[]
    providers: string[]
    mode: 'stable' | 'quick'
  }
  progress: ScanProgress
  started_at: string
  completed_at?: string
  error?: string
  results: NodeResult[]
}

export interface ScanProgress {
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
  candidate_count: number
  batch_size: number
  batch_count: number
  samples_per_node: number
  probe_requests: number
}

export interface NodeSummary {
  name: string
  provider?: string
  protocol?: string
  inferred_region?: string
  region_source: string
}

export interface SwitchEvent {
  id: number
  scan_id: string
  group: string
  previous?: string
  selected: string
  reason: string
  created_at: string
}
