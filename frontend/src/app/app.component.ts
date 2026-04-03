import { Component, OnInit, inject, ChangeDetectionStrategy } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { NavbarComponent } from './core/components/navbar/navbar.component';
import { AuthService } from './core/services/auth.service';
import { ToastModule } from 'primeng/toast';
import { MessageService } from 'primeng/api';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, NavbarComponent, ToastModule],
  providers: [MessageService],
  template: `
    <p-toast />
    @if (auth.isAuthenticated()) {
      <app-navbar />
    }
    <main class="flex flex-col" style="min-height: calc(100vh - 64px);">
      <router-outlet />
    </main>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class AppComponent implements OnInit {
  auth = inject(AuthService);

  ngOnInit(): void {
    this.auth.initialize();
  }
}
