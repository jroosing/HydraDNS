import { ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';
import { ErrorAlertComponent } from '../../../../shared/error-alert/error-alert';
import { LoadingSpinnerComponent } from '../../../../shared/loading-spinner/loading-spinner';
import { FilteringStore } from '../../../../stores/filtering.store';

@Component({
  selector: 'app-filtering-blocklists',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [LoadingSpinnerComponent, ErrorAlertComponent],
  templateUrl: './blocklists.html',
  styleUrl: './filtering.scss',
})
export class FilteringBlocklistsComponent implements OnInit {
  protected readonly store = inject(FilteringStore);

  ngOnInit(): void {
    this.store.loadBlocklists();
  }
}
