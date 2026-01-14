import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';

@Component({
  selector: 'app-error-alert',
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './error-alert.html',
  styleUrl: './error-alert.scss',
})
export class ErrorAlertComponent {
  readonly message = input<string | null>();
  readonly dismiss = output<void>();
}
