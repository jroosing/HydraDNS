import { inject } from '@angular/core';
import { patchState, signalStore, withMethods, withState } from '@ngrx/signals';
import { rxMethod } from '@ngrx/signals/rxjs-interop';
import { interval, pipe, startWith, switchMap, tap } from 'rxjs';
import { ConfigResponse, ServerStats } from '../models/api.models';
import { DnsApiService } from '../services/dns-api.service';

export interface StatsState {
  stats: ServerStats | null;
  config: ConfigResponse | null;
  loading: boolean;
  error: string | null;
}

const initialState: StatsState = {
  stats: null,
  config: null,
  loading: false,
  error: null,
};

export const StatsStore = signalStore(
  { providedIn: 'root' },
  withState(initialState),
  withMethods((store, api = inject(DnsApiService)) => ({
    loadStats: rxMethod<void>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap(() =>
          api.getStats().pipe(
            tap({
              next: (stats) => patchState(store, { stats, loading: false }),
              error: (err) =>
                patchState(store, { error: err.message, loading: false }),
            })
          )
        )
      )
    ),

    loadConfig: rxMethod<void>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap(() =>
          api.getConfig().pipe(
            tap({
              next: (config) => patchState(store, { config, loading: false }),
              error: (err) =>
                patchState(store, { error: err.message, loading: false }),
            })
          )
        )
      )
    ),

    startPolling: rxMethod<number>(
      pipe(
        switchMap((intervalMs) =>
          interval(intervalMs).pipe(
            startWith(0),
            switchMap(() =>
              api.getStats().pipe(
                tap({
                  next: (stats) => patchState(store, { stats }),
                  error: () => {
                    // Silently fail on polling errors
                  },
                })
              )
            )
          )
        )
      )
    ),

    reloadConfig: rxMethod<void>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap(() =>
          api.reloadConfig().pipe(
            tap({
              next: () => patchState(store, { loading: false }),
              error: (err) =>
                patchState(store, { error: err.message, loading: false }),
            })
          )
        )
      )
    ),

    clearError() {
      patchState(store, { error: null });
    },
  }))
);
