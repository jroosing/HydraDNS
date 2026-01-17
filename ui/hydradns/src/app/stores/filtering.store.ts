import { computed, inject } from '@angular/core';
import { patchState, signalStore, withComputed, withMethods, withState } from '@ngrx/signals';
import { rxMethod } from '@ngrx/signals/rxjs-interop';
import { pipe, switchMap, tap } from 'rxjs';
import { FilteringStats, Blocklist } from '../models/api.models';
import { DnsApiService } from '../services/dns-api.service';

export interface FilteringState {
  whitelist: string[];
  blacklist: string[];
  blocklists: Blocklist[];
  stats: FilteringStats | null;
  loading: boolean;
  error: string | null;
}

const initialState: FilteringState = {
  whitelist: [],
  blacklist: [],
  blocklists: [],
  stats: null,
  loading: false,
  error: null,
};

export const FilteringStore = signalStore(
  { providedIn: 'root' },
  withState(initialState),
  withComputed((store) => ({
    isEnabled: computed(() => store.stats()?.enabled ?? false),
    whitelistCount: computed(() => store.whitelist().length),
    blacklistCount: computed(() => store.blacklist().length),
    blocklistCount: computed(() => store.blocklists().length),
    blockedPercentage: computed(() => {
      const stats = store.stats();
      if (!stats || stats.queries_total === 0) return 0;
      return Math.round((stats.queries_blocked / stats.queries_total) * 100);
    }),
  })),
  withMethods((store, api = inject(DnsApiService)) => ({
    loadAll: rxMethod<void>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap(() =>
          api.getFilteringStats().pipe(
            tap({
              next: (stats) => patchState(store, { stats, loading: false }),
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    loadBlocklists: rxMethod<void>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap(() =>
          api.getBlocklists().pipe(
            tap({
              next: (res) => patchState(store, { blocklists: res.blocklists, loading: false }),
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    setBlocklistEnabled: rxMethod<{ name: string; enabled: boolean }>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap(({ name, enabled }) =>
          api.setBlocklistEnabled(name, enabled).pipe(
            switchMap(() => api.getBlocklists()),
            tap({
              next: (res) => patchState(store, { blocklists: res.blocklists, loading: false }),
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    refreshBlocklist: rxMethod<string>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((name) =>
          api.refreshBlocklist(name).pipe(
            switchMap(() => api.getBlocklists()),
            tap({
              next: (res) => patchState(store, { blocklists: res.blocklists, loading: false }),
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    loadWhitelist: rxMethod<void>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap(() =>
          api.getWhitelist().pipe(
            tap({
              next: (res) => patchState(store, { whitelist: res.domains, loading: false }),
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    loadBlacklist: rxMethod<void>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap(() =>
          api.getBlacklist().pipe(
            tap({
              next: (res) => patchState(store, { blacklist: res.domains, loading: false }),
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    addToWhitelist: rxMethod<string[]>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((domains) =>
          api.addWhitelist(domains).pipe(
            tap({
              next: (res) => patchState(store, { whitelist: res.domains, loading: false }),
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    removeFromWhitelist: rxMethod<string[]>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((domains) =>
          api.removeWhitelist(domains).pipe(
            tap({
              next: (res) => patchState(store, { whitelist: res.domains, loading: false }),
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    addToBlacklist: rxMethod<string[]>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((domains) =>
          api.addBlacklist(domains).pipe(
            tap({
              next: (res) => patchState(store, { blacklist: res.domains, loading: false }),
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    removeFromBlacklist: rxMethod<string[]>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((domains) =>
          api.removeBlacklist(domains).pipe(
            tap({
              next: (res) => patchState(store, { blacklist: res.domains, loading: false }),
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    toggleFiltering: rxMethod<boolean>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((enabled) =>
          api.setFilteringEnabled(enabled).pipe(
            tap({
              next: (stats) => patchState(store, { stats, loading: false }),
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    clearError() {
      patchState(store, { error: null });
    },
  })),
);
