import { computed, inject } from '@angular/core';
import { patchState, signalStore, withComputed, withMethods, withState } from '@ngrx/signals';
import { rxMethod } from '@ngrx/signals/rxjs-interop';
import { pipe, switchMap, tap, catchError, of } from 'rxjs';
import { DnsApiService } from '../services/dns-api.service';
import { ConfigResponse } from '../models/api.models';

export interface ConfigState {
  config: ConfigResponse | null;
  loading: boolean;
  saving: boolean;
  error: string | null;
}

const initialState: ConfigState = {
  config: null,
  loading: false,
  saving: false,
  error: null,
};

export const ConfigurationStore = signalStore(
  { providedIn: 'root' },
  withState(initialState),
  withComputed((store) => ({
    upstreamServersText: computed(() => {
      const servers = store.config()?.upstream?.servers || [];
      return servers.join('\n');
    }),
  })),
  withMethods((store, apiService = inject(DnsApiService)) => ({
    loadConfig: rxMethod<void>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap(() =>
          apiService.getConfig().pipe(
            tap((config) => patchState(store, { config, loading: false })),
            catchError((error) => {
              patchState(store, {
                error: error?.error?.error || 'Failed to load configuration',
                loading: false,
              });
              return of(null);
            }),
          ),
        ),
      ),
    ),

    saveConfig: rxMethod<void>(
      pipe(
        tap(() => patchState(store, { saving: true, error: null })),
        switchMap(() => {
          const config = store.config();
          if (!config) {
            patchState(store, { saving: false });
            return of(null);
          }
          return apiService.updateConfig(config).pipe(
            tap(() => patchState(store, { saving: false })),
            catchError((error) => {
              patchState(store, {
                error: error?.error?.error || 'Failed to save configuration',
                saving: false,
              });
              return of(null);
            }),
          );
        }),
      ),
    ),

    updateUpstreamServers(text: string): void {
      const config = store.config();
      if (!config) return;

      const servers = text
        .split('\n')
        .map((s) => s.trim())
        .filter((s) => s.length > 0);

      patchState(store, {
        config: {
          ...config,
          upstream: {
            ...config.upstream,
            servers,
          },
        },
      });
    },

    clearError(): void {
      patchState(store, { error: null });
    },
  })),
);
