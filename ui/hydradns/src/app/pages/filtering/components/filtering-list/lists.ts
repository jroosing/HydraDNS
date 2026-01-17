import { ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';
import { DomainListComponent } from '../../../../shared/domain-list/domain-list';
import { ErrorAlertComponent } from '../../../../shared/error-alert/error-alert';
import { LoadingSpinnerComponent } from '../../../../shared/loading-spinner/loading-spinner';
import { FilteringStore } from '../../../../stores/filtering.store';

@Component({
  selector: 'app-filtering-lists',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [DomainListComponent, LoadingSpinnerComponent, ErrorAlertComponent],
  templateUrl: './lists.html',
  styleUrl: './filtering.scss',
})
export class FilteringListsComponent implements OnInit {
  protected readonly store = inject(FilteringStore);

  ngOnInit(): void {
    this.store.loadWhitelist();
    this.store.loadBlacklist();
  }
}
