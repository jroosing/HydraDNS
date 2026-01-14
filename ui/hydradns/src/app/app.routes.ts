import { Routes } from '@angular/router';
import { LayoutComponent } from './components/layout/layout';

export const routes: Routes = [
  {
    path: '',
    component: LayoutComponent,
    children: [
      {
        path: '',
        redirectTo: 'dashboard',
        pathMatch: 'full',
      },
      {
        path: 'dashboard',
        loadComponent: () =>
          import('./components/dashboard/dashboard').then(
            (m) => m.DashboardComponent
          ),
      },
      {
        path: 'filtering',
        loadComponent: () =>
          import('./components/filtering/filtering').then(
            (m) => m.FilteringComponent
          ),
      },
      {
        path: 'custom-dns',
        loadComponent: () =>
          import('./components/custom-dns/custom-dns').then(
            (m) => m.CustomDnsComponent
          ),
      },
    ],
  },
  {
    path: '**',
    redirectTo: 'dashboard',
  },
];
