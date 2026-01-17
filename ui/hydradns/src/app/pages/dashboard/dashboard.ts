import { ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';
import { ErrorAlertComponent } from '../../shared/error-alert/error-alert';
import { LoadingSpinnerComponent } from '../../shared/loading-spinner/loading-spinner';
import { StatCardComponent } from '../../shared/stat-card/stat-card';
import { StatsStore } from '../../stores/stats.store';
import { FilteringStore } from '../../stores/filtering.store';

@Component({
  selector: 'app-dashboard',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [StatCardComponent, LoadingSpinnerComponent, ErrorAlertComponent],
  templateUrl: './dashboard.html',
  styleUrl: './dashboard.scss',
})
export class DashboardComponent implements OnInit {
  protected readonly statsStore = inject(StatsStore);
  protected readonly filteringStore = inject(FilteringStore);

  ngOnInit(): void {
    this.refresh();
    // Poll stats every 5 seconds
    this.statsStore.startPolling(5000);
  }

  protected refresh(): void {
    this.statsStore.loadStats();
    this.filteringStore.loadAll();
  }
}
