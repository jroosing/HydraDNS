import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    redirectTo: 'dashboard',
    pathMatch: 'full',
  },
  {
    path: 'dashboard',
    loadComponent: () => import('./pages/dashboard/dashboard').then((m) => m.DashboardComponent),
  },
  {
    path: 'filtering',
    loadComponent: () => import('./pages/filtering/filtering').then((m) => m.FilteringComponent),
    children: [
      {
        path: '',
        redirectTo: 'lists',
        pathMatch: 'full',
      },
      {
        path: 'lists',
        loadComponent: () =>
          import('./pages/filtering/components/filtering-list/lists').then(
            (m) => m.FilteringListsComponent,
          ),
      },
      {
        path: 'blocklists',
        loadComponent: () =>
          import('./pages/filtering/components/blocklists/blocklists').then(
            (m) => m.FilteringBlocklistsComponent,
          ),
      },
    ],
  },
  {
    path: 'custom-dns',
    loadComponent: () => import('./pages/custom-dns/custom-dns').then((m) => m.CustomDnsComponent),
  },
  {
    path: '**',
    redirectTo: 'dashboard',
  },
];
