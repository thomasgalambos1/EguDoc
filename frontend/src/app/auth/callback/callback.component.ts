import { Component, OnInit, inject, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ProgressSpinnerModule } from 'primeng/progressspinner';
import { AuthService } from '../../core/services/auth.service';

@Component({
  selector: 'app-callback',
  standalone: true,
  imports: [CommonModule, ProgressSpinnerModule],
  template: `
    <div class="flex items-center justify-center" style="min-height: 100vh;">
      <div class="flex flex-col items-center gap-4">
        <p-progressSpinner strokeWidth="4" />
        <span>Se procesează autentificarea...</span>
      </div>
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class CallbackComponent implements OnInit {
  private auth = inject(AuthService);

  ngOnInit(): void {
    this.auth.handleCallback();
  }
}
