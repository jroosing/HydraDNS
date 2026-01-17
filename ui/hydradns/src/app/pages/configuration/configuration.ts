import { ChangeDetectionStrategy, Component, inject, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ErrorAlertComponent } from '../../shared/error-alert/error-alert';
import { LoadingSpinnerComponent } from '../../shared/loading-spinner/loading-spinner';
import { ConfigurationStore } from '../../stores/configuration.store';

@Component({
  selector: 'app-configuration',
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [FormsModule, LoadingSpinnerComponent, ErrorAlertComponent],
  templateUrl: './configuration.html',
  styleUrl: './configuration.scss',
})
export class ConfigurationComponent implements OnInit {
  protected readonly store = inject(ConfigurationStore);
  protected readonly activeSection = signal<string>('server');

  // Local editable state for upstream servers
  protected readonly upstreamServersText = signal<string>('');

  ngOnInit(): void {
    this.store.loadConfig();
    // Sync the text area with store when config loads
    const upstreamText = this.store.upstreamServersText();
    if (upstreamText) {
      this.upstreamServersText.set(upstreamText);
    }
  }

  protected setActiveSection(section: string): void {
    this.activeSection.set(section);
  }

  protected onUpstreamServersChange(text: string): void {
    this.upstreamServersText.set(text);
    this.store.updateUpstreamServers(text);
  }

  protected saveChanges(): void {
    this.store.saveConfig();
  }
}
