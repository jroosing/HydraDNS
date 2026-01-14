import {
  ChangeDetectionStrategy,
  Component,
  computed,
  inject,
  OnInit,
  signal,
} from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ErrorAlertComponent } from '../error-alert/error-alert';
import { LoadingSpinnerComponent } from '../loading-spinner/loading-spinner';
import { StatCardComponent } from '../stat-card/stat-card';
import { CustomDnsStore } from '../../stores/custom-dns.store';

type TabType = 'hosts' | 'cnames';

@Component({
  selector: 'app-custom-dns',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule, StatCardComponent, LoadingSpinnerComponent, ErrorAlertComponent],
  templateUrl: './custom-dns.html',
  styleUrl: './custom-dns.scss',
})
export class CustomDnsComponent implements OnInit {
  protected readonly store = inject(CustomDnsStore);

  protected readonly activeTab = signal<TabType>('hosts');

  // Host form
  protected readonly newHostName = signal('');
  protected readonly newHostIPs = signal('');

  // CNAME form
  protected readonly newCnameAlias = signal('');
  protected readonly newCnameTarget = signal('');

  // Editing state
  protected readonly editingHostName = signal<string | null>(null);
  protected readonly editingCnameAlias = signal<string | null>(null);

  protected readonly canAddHost = computed(
    () => this.newHostName().trim() && this.newHostIPs().trim()
  );

  protected readonly canAddCname = computed(
    () => this.newCnameAlias().trim() && this.newCnameTarget().trim()
  );

  protected readonly filteredHosts = computed(() => this.store.hostsList());
  protected readonly filteredCnames = computed(() => this.store.cnamesList());

  ngOnInit(): void {
    this.store.loadAll();
  }

  protected handleAddHost(event: Event): void {
    event.preventDefault();
    const name = this.newHostName().trim();
    const ipsString = this.newHostIPs().trim();

    if (name && ipsString) {
      const ips = ipsString.split(',').map((ip) => ip.trim()).filter(Boolean);
      if (ips.length > 0) {
        if (this.editingHostName()) {
          this.store.updateHost({ name, ips });
          this.editingHostName.set(null);
        } else {
          this.store.addHost({ name, ips });
        }
        this.newHostName.set('');
        this.newHostIPs.set('');
      }
    }
  }

  protected handleAddCname(event: Event): void {
    event.preventDefault();
    const alias = this.newCnameAlias().trim();
    const target = this.newCnameTarget().trim();

    if (alias && target) {
      if (this.editingCnameAlias()) {
        this.store.updateCname({ alias, target });
        this.editingCnameAlias.set(null);
      } else {
        this.store.addCname({ alias, target });
      }
      this.newCnameAlias.set('');
      this.newCnameTarget.set('');
    }
  }

  protected editHost(host: { name: string; ips: string[] }): void {
    this.newHostName.set(host.name);
    this.newHostIPs.set(host.ips.join(', '));
    this.editingHostName.set(host.name);
  }

  protected editCname(cname: { alias: string; target: string }): void {
    this.newCnameAlias.set(cname.alias);
    this.newCnameTarget.set(cname.target);
    this.editingCnameAlias.set(cname.alias);
  }
}
