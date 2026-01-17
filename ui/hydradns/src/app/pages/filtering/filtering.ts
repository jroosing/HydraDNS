import { ChangeDetectionStrategy, Component, effect, inject, OnInit, signal } from '@angular/core';
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
  private refreshCooldownUntil = 0;
  private awaitingRefresh = false;
  protected readonly showToast = signal(false);

  constructor() {
    // Show a brief toast when a manual refresh completes
    effect(() => {
      const loading = this.store.loading();
      if (this.awaitingRefresh && !loading) {
        this.showToast.set(true);
        this.awaitingRefresh = false;
        // Hide after 2s
        setTimeout(() => this.showToast.set(false), 2000);
      }
    });
  }

  ngOnInit(): void {
    this.refreshStats();
  }

  protected toggleFiltering(): void {
    this.store.toggleFiltering(!this.store.isEnabled());
  }

  protected refreshStats(): void {
    const now = Date.now();
    if (now < this.refreshCooldownUntil) {
      return;
    }
    this.refreshCooldownUntil = now + 1000; // 1s debounce
    this.awaitingRefresh = true;
    this.store.loadAll();
  }

  protected cooldownActive(): boolean {
    return Date.now() < this.refreshCooldownUntil;
  }
}
