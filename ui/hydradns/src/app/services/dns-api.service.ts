import { HttpClient } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import {
  CNAMERecord,
  ConfigResponse,
  CustomDNSOperationResponse,
  CustomDNSRecords,
  DomainListResponse,
  DomainRequest,
  FilteringStats,
  HostRecord,
  ServerStats,
  UpdateCNAMERequest,
  UpdateHostRequest,
} from '../models/api.models';

@Injectable({ providedIn: 'root' })
export class DnsApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = '/api/v1';

  // Health & Stats
  getHealth(): Observable<{ status: string }> {
    return this.http.get<{ status: string }>(`${this.baseUrl}/health`);
  }

  getStats(): Observable<ServerStats> {
    return this.http.get<ServerStats>(`${this.baseUrl}/stats`);
  }

  // Config
  getConfig(): Observable<ConfigResponse> {
    return this.http.get<ConfigResponse>(`${this.baseUrl}/config`);
  }

  updateConfig(config: Partial<ConfigResponse>): Observable<ConfigResponse> {
    return this.http.put<ConfigResponse>(`${this.baseUrl}/config`, config);
  }

  reloadConfig(): Observable<{ message: string }> {
    return this.http.post<{ message: string }>(`${this.baseUrl}/config/reload`, {});
  }

  // Filtering - Whitelist
  getWhitelist(): Observable<DomainListResponse> {
    return this.http.get<DomainListResponse>(`${this.baseUrl}/filtering/whitelist`);
  }

  addWhitelist(domains: string[]): Observable<DomainListResponse> {
    return this.http.post<DomainListResponse>(`${this.baseUrl}/filtering/whitelist`, {
      domains,
    } as DomainRequest);
  }

  removeWhitelist(domains: string[]): Observable<DomainListResponse> {
    return this.http.delete<DomainListResponse>(`${this.baseUrl}/filtering/whitelist`, {
      body: { domains } as DomainRequest,
    });
  }

  // Filtering - Blacklist
  getBlacklist(): Observable<DomainListResponse> {
    return this.http.get<DomainListResponse>(`${this.baseUrl}/filtering/blacklist`);
  }

  addBlacklist(domains: string[]): Observable<DomainListResponse> {
    return this.http.post<DomainListResponse>(`${this.baseUrl}/filtering/blacklist`, {
      domains,
    } as DomainRequest);
  }

  removeBlacklist(domains: string[]): Observable<DomainListResponse> {
    return this.http.delete<DomainListResponse>(`${this.baseUrl}/filtering/blacklist`, {
      body: { domains } as DomainRequest,
    });
  }

  // Filtering Stats & Toggle
  getFilteringStats(): Observable<FilteringStats> {
    return this.http.get<FilteringStats>(`${this.baseUrl}/filtering/stats`);
  }

  setFilteringEnabled(enabled: boolean): Observable<FilteringStats> {
    return this.http.put<FilteringStats>(`${this.baseUrl}/filtering/enabled`, { enabled });
  }

  // Custom DNS
  getCustomDNS(): Observable<CustomDNSRecords> {
    return this.http.get<CustomDNSRecords>(`${this.baseUrl}/custom-dns`);
  }

  // Hosts
  addHost(host: HostRecord): Observable<CustomDNSOperationResponse> {
    return this.http.post<CustomDNSOperationResponse>(`${this.baseUrl}/custom-dns/hosts`, host);
  }

  updateHost(name: string, ips: string[]): Observable<CustomDNSOperationResponse> {
    return this.http.put<CustomDNSOperationResponse>(`${this.baseUrl}/custom-dns/hosts/${name}`, {
      ips,
    } as UpdateHostRequest);
  }

  deleteHost(name: string): Observable<CustomDNSOperationResponse> {
    return this.http.delete<CustomDNSOperationResponse>(`${this.baseUrl}/custom-dns/hosts/${name}`);
  }

  // CNAMEs
  addCNAME(cname: CNAMERecord): Observable<CustomDNSOperationResponse> {
    return this.http.post<CustomDNSOperationResponse>(`${this.baseUrl}/custom-dns/cnames`, cname);
  }

  updateCNAME(alias: string, target: string): Observable<CustomDNSOperationResponse> {
    return this.http.put<CustomDNSOperationResponse>(
      `${this.baseUrl}/custom-dns/cnames/${alias}`,
      { target } as UpdateCNAMERequest
    );
  }

  deleteCNAME(alias: string): Observable<CustomDNSOperationResponse> {
    return this.http.delete<CustomDNSOperationResponse>(
      `${this.baseUrl}/custom-dns/cnames/${alias}`
    );
  }
}
