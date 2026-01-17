// API Response Models matching Go backend

export interface FilteringStats {
  enabled: boolean;
  queries_total: number;
  queries_blocked: number;
  queries_allowed: number;
  whitelist_size: number;
  blacklist_size: number;
}

export interface DomainListResponse {
  domains: string[];
  count: number;
}

export interface DomainRequest {
  domains: string[];
}

export interface FilteringEnabledRequest {
  enabled: boolean;
}

export interface CustomDNSCounts {
  hosts: number;
  cnames: number;
  total: number;
}

export interface CustomDNSRecords {
  hosts: Record<string, string[]>;
  cnames: Record<string, string>;
  count: CustomDNSCounts;
}

export interface HostRecord {
  name: string;
  ips: string[];
}

export interface CNAMERecord {
  alias: string;
  target: string;
}

export interface UpdateHostRequest {
  ips: string[];
}

export interface UpdateCNAMERequest {
  target: string;
}

export interface CustomDNSOperationResponse {
  message: string;
  data?: unknown;
}

export interface DNSStats {
  queries_total: number;
  queries_udp: number;
  queries_tcp: number;
  responses_nxdomain: number;
  responses_error: number;
  avg_latency_ms: number;
}

export interface ServerStats {
  uptime: string;
  uptime_seconds: number;
  start_time: string;
  goroutines: number;
  memory_alloc_mb: number;
  num_cpu: number;
  dns: DNSStats;
  filtering?: FilteringStats;
}

export interface APIConfig {
  enabled: boolean;
  host: string;
  port: number;
}

export interface ServerConfig {
  host: string;
  port: number;
  workers: string;
  max_concurrency: number;
  upstream_socket_pool_size: number;
  enable_tcp: boolean;
  tcp_fallback: boolean;
}

export interface UpstreamConfig {
  servers: string[];
  timeout: string;
  retries: number;
}

export interface LoggingConfig {
  level: string;
  format: string;
}

export interface FilteringConfig {
  enabled: boolean;
  whitelist_file: string;
  blacklist_file: string;
  blocklist_urls: string[];
  refresh_interval: string;
}

export interface Blocklist {
  name: string;
  url: string;
  format: string;
  enabled: boolean;
  last_fetched?: string;
}

export interface BlocklistsResponse {
  blocklists: Blocklist[];
  count: number;
}

export interface RateLimitConfig {
  enabled: boolean;
  requests_per_second: number;
  burst: number;
}

export interface CustomDNSConfig {
  enabled: boolean;
  hosts_file: string;
  inline_hosts: Record<string, string[]>;
  inline_cnames: Record<string, string>;
}

export interface ConfigResponse {
  server: ServerConfig;
  upstream: UpstreamConfig;
  custom_dns: CustomDNSConfig;
  logging: LoggingConfig;
  filtering: FilteringConfig;
  rate_limit: RateLimitConfig;
  api: APIConfig;
}
