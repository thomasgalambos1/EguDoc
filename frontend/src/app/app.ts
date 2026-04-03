import { Component, OnInit, inject, ChangeDetectionStrategy } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { CommonModule } from '@angular/common';
import { ToastModule } from 'primeng/toast';
import { MessageService } from 'primeng/api';
import { NavbarComponent } from './core/components/navbar/navbar.component';
import { AuthService } from './core/services/auth.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [RouterOutlet, NavbarComponent, ToastModule, CommonModule],
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
export class App implements OnInit {
  auth = inject(AuthService);

  ngOnInit(): void {
    this.auth.initialize();
  }
}
