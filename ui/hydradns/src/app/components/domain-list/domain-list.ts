import {
  ChangeDetectionStrategy,
  Component,
  computed,
  input,
  output,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';

@Component({
  selector: 'app-domain-list',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule],
  templateUrl: './domain-list.html',
  styleUrl: './domain-list.scss',
})
export class DomainListComponent {
  readonly title = input.required<string>();
  readonly domains = input.required<string[]>();

  readonly addDomain = output<string[]>();
  readonly removeDomain = output<string[]>();

  protected readonly searchQuery = signal('');
  protected readonly newDomain = signal('');

  protected readonly filteredDomains = computed(() => {
    const query = this.searchQuery().toLowerCase();
    if (!query) return this.domains();
    return this.domains().filter((d) => d.toLowerCase().includes(query));
  });

  protected handleAddDomain(event: Event): void {
    event.preventDefault();
    const domain = this.newDomain().trim();
    if (domain) {
      this.addDomain.emit([domain]);
      this.newDomain.set('');
    }
  }
}
