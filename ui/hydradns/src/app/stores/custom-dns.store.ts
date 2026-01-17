import { computed, inject } from '@angular/core';
import { patchState, signalStore, withComputed, withMethods, withState } from '@ngrx/signals';
import { rxMethod } from '@ngrx/signals/rxjs-interop';
import { pipe, switchMap, tap } from 'rxjs';
import { CNAMERecord, HostRecord } from '../models/api.models';
import { DnsApiService } from '../services/dns-api.service';

export interface CustomDnsState {
  hosts: Record<string, string[]>;
  cnames: Record<string, string>;
  loading: boolean;
  error: string | null;
}

const initialState: CustomDnsState = {
  hosts: {},
  cnames: {},
  loading: false,
  error: null,
};

export const CustomDnsStore = signalStore(
  { providedIn: 'root' },
  withState(initialState),
  withComputed((store) => ({
    hostsList: computed(() => Object.entries(store.hosts()).map(([name, ips]) => ({ name, ips }))),
    cnamesList: computed(() =>
      Object.entries(store.cnames()).map(([alias, target]) => ({ alias, target })),
    ),
    hostsCount: computed(() => Object.keys(store.hosts()).length),
    cnamesCount: computed(() => Object.keys(store.cnames()).length),
    totalCount: computed(
      () => Object.keys(store.hosts()).length + Object.keys(store.cnames()).length,
    ),
  })),
  withMethods((store, api = inject(DnsApiService)) => ({
    loadAll: rxMethod<void>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap(() =>
          api.getCustomDNS().pipe(
            tap({
              next: (res) =>
                patchState(store, {
                  hosts: res.hosts,
                  cnames: res.cnames,
                  loading: false,
                }),
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    addHost: rxMethod<HostRecord>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((host) =>
          api.addHost(host).pipe(
            tap({
              next: () => {
                patchState(store, (state) => ({
                  hosts: { ...state.hosts, [host.name]: host.ips },
                  loading: false,
                }));
              },
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    updateHost: rxMethod<HostRecord>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((host) =>
          api.updateHost(host.name, host.ips).pipe(
            tap({
              next: () => {
                patchState(store, (state) => ({
                  hosts: { ...state.hosts, [host.name]: host.ips },
                  loading: false,
                }));
              },
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    deleteHost: rxMethod<string>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((name) =>
          api.deleteHost(name).pipe(
            tap({
              next: () => {
                patchState(store, (state) => {
                  const hosts = { ...state.hosts };
                  delete hosts[name];
                  return { hosts, loading: false };
                });
              },
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    addCname: rxMethod<CNAMERecord>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((cname) =>
          api.addCNAME(cname).pipe(
            tap({
              next: () => {
                patchState(store, (state) => ({
                  cnames: { ...state.cnames, [cname.alias]: cname.target },
                  loading: false,
                }));
              },
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    updateCname: rxMethod<CNAMERecord>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((cname) =>
          api.updateCNAME(cname.alias, cname.target).pipe(
            tap({
              next: () => {
                patchState(store, (state) => ({
                  cnames: { ...state.cnames, [cname.alias]: cname.target },
                  loading: false,
                }));
              },
              error: (err) => patchState(store, { error: err.message, loading: false }),
            }),
          ),
        ),
      ),
    ),

    deleteCname: rxMethod<string>(
      pipe(
        tap(() => patchState(store, { loading: true, error: null })),
        switchMap((alias) =>
          api.deleteCNAME(alias).pipe(
            tap({
              next: () => {
                patchState(store, (state) => {
                  const cnames = { ...state.cnames };
                  delete cnames[alias];
                  return { cnames, loading: false };
                });
              },
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
