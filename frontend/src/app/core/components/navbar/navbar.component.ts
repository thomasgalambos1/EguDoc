import { Component, inject, ChangeDetectionStrategy } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { CommonModule } from '@angular/common';
import { MenubarModule } from 'primeng/menubar';
import { ButtonModule } from 'primeng/button';
import { AvatarModule } from 'primeng/avatar';
import { TooltipModule } from 'primeng/tooltip';
import { MenuItem } from 'primeng/api';
import { AuthService } from '../../services/auth.service';

@Component({
  selector: 'app-navbar',
  standalone: true,
  imports: [CommonModule, MenubarModule, ButtonModule, AvatarModule, TooltipModule, RouterLink],
  template: `
    <p-menubar [model]="menuItems">
      <ng-template #start>
        <span class="font-bold text-xl mr-6" routerLink="/dashboard" style="cursor: pointer;">
          EguDoc
        </span>
      </ng-template>
      <ng-template #end>
        <div class="flex items-center gap-3">
          <span class="text-sm">{{ auth.userInfo()?.email }}</span>
          <p-avatar [label]="userInitials()" shape="circle" size="normal" />
          <p-button
            icon="pi pi-sign-out"
            severity="secondary"
            [rounded]="true"
            [text]="true"
            (onClick)="auth.logout()"
            pTooltip="Deconectare"
          />
        </div>
      </ng-template>
    </p-menubar>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class NavbarComponent {
  auth = inject(AuthService);
  router = inject(Router);

  get menuItems(): MenuItem[] {
    return [
      { label: 'Registratură', icon: 'pi pi-inbox', routerLink: '/registratura' },
      { label: 'Entități', icon: 'pi pi-users', routerLink: '/entitati' },
      { label: 'Registre', icon: 'pi pi-book', routerLink: '/registre' },
      ...(this.auth.hasAnyRole(['superadmin', 'institution_admin']) ? [
        { label: 'Administrare', icon: 'pi pi-cog', routerLink: '/admin' }
      ] : [])
    ];
  }

  userInitials(): string {
    const info = this.auth.userInfo();
    if (!info) return '?';
    if (info.given_name && info.family_name) {
      return (info.given_name[0] + info.family_name[0]).toUpperCase();
    }
    return info.email[0].toUpperCase();
  }
}
