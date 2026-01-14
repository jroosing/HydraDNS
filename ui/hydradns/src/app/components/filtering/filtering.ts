import { ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';
import { DomainListComponent } from '../domain-list/domain-list';
import { ErrorAlertComponent } from '../error-alert/error-alert';
import { LoadingSpinnerComponent } from '../loading-spinner/loading-spinner';
import { StatCardComponent } from '../stat-card/stat-card';
import { FilteringStore } from '../../stores/filtering.store';

@Component({
  selector: 'app-filtering',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [DomainListComponent, StatCardComponent, LoadingSpinnerComponent, ErrorAlertComponent],
  templateUrl: './filtering.html',
  styleUrl: './filtering.scss',
})
export class FilteringComponent implements OnInit {
  protected readonly store = inject(FilteringStore);

  ngOnInit(): void {
    this.store.loadAll();
    this.store.loadWhitelist();
    this.store.loadBlacklist();
  }

  protected toggleFiltering(): void {
    this.store.toggleFiltering(!this.store.isEnabled());
  }
}
