import { ChangeDetectionStrategy, Component, inject, OnInit } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { StatCardComponent } from '../../shared/stat-card/stat-card';
import { FilteringStore } from '../../stores/filtering.store';

@Component({
  selector: 'app-filtering',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, StatCardComponent],
  templateUrl: './filtering.html',
  styleUrl: './filtering.scss',
})
export class FilteringComponent implements OnInit {
  protected readonly store = inject(FilteringStore);

  ngOnInit(): void {
    this.store.loadAll();
  }

  protected toggleFiltering(): void {
    this.store.toggleFiltering(!this.store.isEnabled());
  }
}
