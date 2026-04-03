# EguDoc — Sub-plan C: Frontend (Angular 21 + PrimeNG 21)

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`. Depends on Sub-plans A and B. Read CLAUDE.md frontend rules carefully before writing any code.

**Goal:** Build a complete, professional Angular 21 SPA using PrimeNG 21 components exclusively for UI, TailwindCSS for layout only, with full OIDC auth (eguilde), granular RBAC reflected in the UI, and complete screens for registratura, workflow, entities, registries, and admin.

**Architecture:** Standalone Angular 21 components, lazy-loaded route-based feature modules, typed reactive forms, OnPush change detection throughout. Auth via OIDC Authorization Code + PKCE flow against eguilde. No layout shell components — each route is a self-contained feature component. Token stored in memory (access token) + httpOnly cookie (refresh token via eguilde pattern).

**Tech Stack:** Angular 21, PrimeNG 21, TailwindCSS 4, `angular-oauth2-oidc` library, `@angular/signals`

**Critical rules (non-negotiable):**
- ALL UI components from PrimeNG 21 — no custom buttons, inputs, tables, dialogs
- TailwindCSS ONLY for `flex`, `gap`, `grid`, `p-*`, `m-*`, `w-*`, `h-*` — NEVER for colors
- ALL colors from PrimeNG theme — never Tailwind color classes (`text-red-500`, `bg-blue-200`, etc.)
- Standalone components, typed reactive forms, OnPush everywhere
- NO layout/shell/wrapper components — route directly to feature components
- Angular routing with lazy-loaded routes for every feature module

---

## File Map

```
frontend/
├── package.json
├── angular.json
├── tailwind.config.js
├── src/
│   ├── main.ts
│   ├── styles.scss
│   ├── app/
│   │   ├── app.config.ts              # provideRouter, provideAnimationsAsync, OIDC, HTTP
│   │   ├── app.routes.ts              # top-level lazy routes
│   │   ├── core/
│   │   │   ├── guards/
│   │   │   │   ├── auth.guard.ts      # redirects to OIDC login if not authenticated
│   │   │   │   └── role.guard.ts     # checks user has required role
│   │   │   ├── interceptors/
│   │   │   │   └── auth.interceptor.ts # adds Bearer token to all /api calls
│   │   │   ├── services/
│   │   │   │   ├── auth.service.ts   # wraps angular-oauth2-oidc, exposes user signal
│   │   │   │   ├── registratura.service.ts
│   │   │   │   ├── entities.service.ts
│   │   │   │   ├── registry.service.ts
│   │   │   │   ├── workflow.service.ts
│   │   │   │   ├── users.service.ts
│   │   │   │   └── admin.service.ts
│   │   │   └── models/
│   │   │       ├── document.model.ts
│   │   │       ├── entity.model.ts
│   │   │       ├── user.model.ts
│   │   │       └── api.model.ts       # PaginatedResponse, ApiError
│   │   ├── auth/
│   │   │   ├── callback/
│   │   │   │   └── callback.component.ts  # handles OIDC redirect + code exchange
│   │   │   └── silent-refresh/
│   │   │       └── silent-refresh.component.ts
│   │   ├── registratura/
│   │   │   ├── registratura.routes.ts
│   │   │   ├── list/
│   │   │   │   └── document-list.component.ts    # paginated, filterable document list
│   │   │   ├── detail/
│   │   │   │   └── document-detail.component.ts  # full document view + attachments + audit
│   │   │   ├── create/
│   │   │   │   └── document-create.component.ts  # registration form
│   │   │   └── workflow/
│   │   │       └── workflow-actions.component.ts # assign, approve, reject actions panel
│   │   ├── entities/
│   │   │   ├── entities.routes.ts
│   │   │   ├── list/
│   │   │   │   └── entities-list.component.ts
│   │   │   └── form/
│   │   │       └── entity-form.component.ts      # create/edit persoana/firma/institutie
│   │   ├── registries/
│   │   │   ├── registries.routes.ts
│   │   │   └── list/
│   │   │       └── registries-list.component.ts
│   │   ├── dashboard/
│   │   │   └── dashboard.component.ts            # stats, recent documents, pending approvals
│   │   └── admin/
│   │       ├── admin.routes.ts
│   │       ├── users/
│   │       │   └── admin-users.component.ts
│   │       ├── roles/
│   │       │   └── admin-roles.component.ts
│   │       └── institutions/
│   │           └── admin-institutions.component.ts
│   └── environments/
│       ├── environment.ts
│       └── environment.production.ts
```

---

## Task 15: Angular project scaffold

**Files:**
- Create: `frontend/` (Angular project via CLI)

- [ ] **Step 15.1: Create Angular app**

```bash
cd /c/dev/egudoc
npx @angular/cli@latest new frontend \
  --standalone \
  --routing \
  --style=scss \
  --skip-git \
  --skip-tests=false
cd frontend
```

- [ ] **Step 15.2: Install PrimeNG 21 and dependencies**

```bash
npm install primeng@21 primeicons @primeng/themes
npm install tailwindcss @tailwindcss/postcss postcss
npm install angular-oauth2-oidc
npm install --save-dev @types/node
```

- [ ] **Step 15.3: Configure tailwind.config.js**

```js
// frontend/tailwind.config.js
const { createGUIOptions } = require('tailwindcss/lib/util/createUtilityPlugin');

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./src/**/*.{html,ts,scss}"
  ],
  // No theme extension — colors come from PrimeNG theme exclusively
  theme: {
    extend: {}
  },
  plugins: [],
  // Disable color utilities to prevent accidental usage
  corePlugins: {
    // Layout utilities we DO want
    display: true,
    flexDirection: true,
    flexWrap: true,
    flex: true,
    flexGrow: true,
    flexShrink: true,
    alignItems: true,
    alignSelf: true,
    justifyContent: true,
    justifyItems: true,
    justifySelf: true,
    gap: true,
    padding: true,
    margin: true,
    width: true,
    height: true,
    minWidth: true,
    minHeight: true,
    maxWidth: true,
    maxHeight: true,
    overflow: true,
    position: true,
    inset: true,
    zIndex: true,
    grid: true,
    gridTemplateColumns: true,
    gridColumn: true,
    gridTemplateRows: true,
    gridRow: true,
    // Colors DISABLED — use PrimeNG theme tokens only
    backgroundColor: false,
    textColor: false,
    borderColor: false,
    placeholderColor: false,
    ringColor: false,
    gradientColorStops: false,
  }
};
```

- [ ] **Step 15.4: Update styles.scss**

```scss
/* frontend/src/styles.scss */
@import "tailwindcss";

/* PrimeNG Aura theme — all colors come from here */
@import "primeng/resources/themes/aura-light-blue/theme.css";
@import "primeng/resources/primeng.css";
@import "primeicons/primeicons.css";

/* Global layout resets only — no custom colors */
html, body {
  height: 100%;
  margin: 0;
  padding: 0;
  font-family: var(--font-family);
}

/* Ensure full-height routing outlet */
app-root {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}
```

- [ ] **Step 15.5: Commit**

```bash
cd /c/dev/egudoc
git add frontend/
git commit -m "feat: scaffold Angular 21 frontend with PrimeNG 21 and Tailwind"
```

---

## Task 16: OIDC authentication setup

**Files:**
- Create: `frontend/src/app/app.config.ts`
- Create: `frontend/src/app/app.routes.ts`
- Create: `frontend/src/app/core/services/auth.service.ts`
- Create: `frontend/src/app/core/guards/auth.guard.ts`
- Create: `frontend/src/app/core/interceptors/auth.interceptor.ts`
- Create: `frontend/src/app/auth/callback/callback.component.ts`
- Create: `frontend/src/environments/environment.ts`

- [ ] **Step 16.1: Write environment.ts**

```typescript
// frontend/src/environments/environment.ts
export const environment = {
  production: false,
  apiUrl: 'http://localhost:8090',
  oidc: {
    issuer: 'http://localhost:3100/api/oidc',
    clientId: 'egudoc-spa',
    redirectUri: 'http://localhost:4200/auth/callback',
    scope: 'openid profile email offline_access',
    responseType: 'code',
    useSilentRefresh: false,
    showDebugInformation: true,
    requireHttps: false,
    // PKCE — mandatory (eguilde requires S256)
    usePkce: true,
    pkceMethod: 'S256' as const,
  }
};
```

- [ ] **Step 16.2: Write app.config.ts**

```typescript
// frontend/src/app/app.config.ts
import { ApplicationConfig, importProvidersFrom } from '@angular/core';
import { provideRouter, withComponentInputBinding, withViewTransitions } from '@angular/router';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { OAuthModule } from 'angular-oauth2-oidc';

import { routes } from './app.routes';
import { authInterceptor } from './core/interceptors/auth.interceptor';

export const appConfig: ApplicationConfig = {
  providers: [
    provideRouter(routes, withComponentInputBinding(), withViewTransitions()),
    provideHttpClient(withInterceptors([authInterceptor])),
    provideAnimationsAsync(),
    importProvidersFrom(OAuthModule.forRoot()),
  ]
};
```

- [ ] **Step 16.3: Write app.routes.ts**

```typescript
// frontend/src/app/app.routes.ts
import { Routes } from '@angular/router';
import { authGuard } from './core/guards/auth.guard';
import { roleGuard } from './core/guards/role.guard';

export const routes: Routes = [
  {
    path: 'auth/callback',
    loadComponent: () => import('./auth/callback/callback.component').then(m => m.CallbackComponent)
  },
  {
    path: 'dashboard',
    loadComponent: () => import('./dashboard/dashboard.component').then(m => m.DashboardComponent),
    canActivate: [authGuard]
  },
  {
    path: 'registratura',
    loadChildren: () => import('./registratura/registratura.routes').then(m => m.REGISTRATURA_ROUTES),
    canActivate: [authGuard]
  },
  {
    path: 'entitati',
    loadChildren: () => import('./entities/entities.routes').then(m => m.ENTITIES_ROUTES),
    canActivate: [authGuard]
  },
  {
    path: 'registre',
    loadChildren: () => import('./registries/registries.routes').then(m => m.REGISTRIES_ROUTES),
    canActivate: [authGuard]
  },
  {
    path: 'admin',
    loadChildren: () => import('./admin/admin.routes').then(m => m.ADMIN_ROUTES),
    canActivate: [authGuard, roleGuard(['superadmin', 'institution_admin'])]
  },
  {
    path: '',
    redirectTo: 'dashboard',
    pathMatch: 'full'
  },
  {
    path: '**',
    redirectTo: 'dashboard'
  }
];
```

- [ ] **Step 16.4: Write auth.service.ts**

```typescript
// frontend/src/app/core/services/auth.service.ts
import { Injectable, inject, signal, computed } from '@angular/core';
import { Router } from '@angular/router';
import { OAuthService, AuthConfig } from 'angular-oauth2-oidc';
import { environment } from '../../../environments/environment';

export interface UserInfo {
  sub: string;
  email: string;
  given_name?: string;
  family_name?: string;
  roles?: string[];
}

@Injectable({ providedIn: 'root' })
export class AuthService {
  private oauth = inject(OAuthService);
  private router = inject(Router);

  private _userInfo = signal<UserInfo | null>(null);

  readonly userInfo = this._userInfo.asReadonly();
  readonly isAuthenticated = computed(() => this._userInfo() !== null);
  readonly userRoles = computed(() => this._userInfo()?.roles ?? []);

  async initialize(): Promise<void> {
    const config: AuthConfig = {
      issuer: environment.oidc.issuer,
      clientId: environment.oidc.clientId,
      redirectUri: environment.oidc.redirectUri,
      scope: environment.oidc.scope,
      responseType: environment.oidc.responseType,
      usePkce: true,
      requireHttps: environment.oidc.requireHttps,
      showDebugInformation: environment.oidc.showDebugInformation,
      clearHashAfterLogin: true,
    };

    this.oauth.configure(config);

    try {
      await this.oauth.loadDiscoveryDocumentAndTryLogin();
      if (this.oauth.hasValidAccessToken()) {
        await this.loadUserInfo();
      }
    } catch (err) {
      console.error('OIDC initialization error', err);
    }

    // Auto-refresh: attempt silent refresh before expiry
    this.oauth.setupAutomaticSilentRefresh();
  }

  login(): void {
    this.oauth.initCodeFlow();
  }

  async handleCallback(): Promise<void> {
    try {
      await this.oauth.loadDiscoveryDocumentAndTryLogin();
      if (this.oauth.hasValidAccessToken()) {
        await this.loadUserInfo();
        this.router.navigate(['/dashboard']);
      }
    } catch (err) {
      console.error('OAuth callback error', err);
      this.login();
    }
  }

  logout(): void {
    this.oauth.logOut();
    this._userInfo.set(null);
  }

  getAccessToken(): string {
    return this.oauth.getAccessToken();
  }

  hasRole(role: string): boolean {
    return this.userRoles().includes(role);
  }

  hasAnyRole(roles: string[]): boolean {
    return roles.some(r => this.hasRole(r));
  }

  private async loadUserInfo(): Promise<void> {
    try {
      const info = await this.oauth.loadUserProfile() as any;
      this._userInfo.set({
        sub: info.sub,
        email: info.email,
        given_name: info.given_name,
        family_name: info.family_name,
        roles: info.roles ?? [],
      });
    } catch (err) {
      console.error('Failed to load user info', err);
    }
  }
}
```

- [ ] **Step 16.5: Write auth.guard.ts**

```typescript
// frontend/src/app/core/guards/auth.guard.ts
import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthService } from '../services/auth.service';

export const authGuard: CanActivateFn = () => {
  const auth = inject(AuthService);
  const router = inject(Router);

  if (auth.isAuthenticated()) {
    return true;
  }

  // Trigger OIDC login flow
  auth.login();
  return false;
};
```

- [ ] **Step 16.6: Write role.guard.ts**

```typescript
// frontend/src/app/core/guards/role.guard.ts
import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthService } from '../services/auth.service';

export function roleGuard(requiredRoles: string[]): CanActivateFn {
  return () => {
    const auth = inject(AuthService);
    const router = inject(Router);

    if (auth.hasAnyRole(requiredRoles)) {
      return true;
    }

    // Redirect to dashboard with an error — the user is authenticated but lacks the role
    router.navigate(['/dashboard'], { queryParams: { error: 'access_denied' } });
    return false;
  };
}
```

- [ ] **Step 16.7: Write auth.interceptor.ts**

```typescript
// frontend/src/app/core/interceptors/auth.interceptor.ts
import { HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { AuthService } from '../services/auth.service';
import { environment } from '../../../environments/environment';

export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const auth = inject(AuthService);

  // Only add auth header to our own API
  if (!req.url.startsWith(environment.apiUrl)) {
    return next(req);
  }

  const token = auth.getAccessToken();
  if (!token) {
    return next(req);
  }

  const authReq = req.clone({
    setHeaders: { Authorization: `Bearer ${token}` }
  });
  return next(authReq);
};
```

- [ ] **Step 16.8: Write callback.component.ts**

```typescript
// frontend/src/app/auth/callback/callback.component.ts
import { Component, OnInit, inject } from '@angular/core';
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
```

*(Add `import { ChangeDetectionStrategy } from '@angular/core';` at the top.)*

- [ ] **Step 16.9: Commit**

```bash
cd /c/dev/egudoc
git add frontend/src/
git commit -m "feat: add OIDC auth with angular-oauth2-oidc, guards, interceptor"
```

---

## Task 17: Navigation component and app shell via routing

**Files:**
- Create: `frontend/src/app/app.component.ts`
- Create: `frontend/src/app/core/components/navbar/navbar.component.ts`

- [ ] **Step 17.1: Write app.component.ts**

```typescript
// frontend/src/app/app.component.ts
import { Component, OnInit, inject, ChangeDetectionStrategy } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { NavbarComponent } from './core/components/navbar/navbar.component';
import { AuthService } from './core/services/auth.service';
import { ToastModule } from 'primeng/toast';
import { MessageService } from 'primeng/api';
import { CommonModule } from '@angular/common';

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
export class AppComponent implements OnInit {
  auth = inject(AuthService);

  ngOnInit(): void {
    this.auth.initialize();
  }
}
```

- [ ] **Step 17.2: Write navbar.component.ts**

```typescript
// frontend/src/app/core/components/navbar/navbar.component.ts
import { Component, inject, ChangeDetectionStrategy } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { CommonModule } from '@angular/common';
import { MenubarModule } from 'primeng/menubar';
import { ButtonModule } from 'primeng/button';
import { AvatarModule } from 'primeng/avatar';
import { MenuItem } from 'primeng/api';
import { AuthService } from '../../services/auth.service';

@Component({
  selector: 'app-navbar',
  standalone: true,
  imports: [CommonModule, MenubarModule, ButtonModule, AvatarModule, RouterLink],
  template: `
    <p-menubar [model]="menuItems" styleClass="border-noround px-4">
      <ng-template pTemplate="start">
        <span class="font-bold text-xl mr-6" routerLink="/dashboard" style="cursor: pointer;">
          EguDoc
        </span>
      </ng-template>
      <ng-template pTemplate="end">
        <div class="flex items-center gap-3">
          <span class="text-sm">{{ auth.userInfo()?.email }}</span>
          <p-avatar
            [label]="userInitials()"
            shape="circle"
            size="normal"
          />
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

  menuItems: MenuItem[] = [
    {
      label: 'Registratură',
      icon: 'pi pi-inbox',
      routerLink: '/registratura'
    },
    {
      label: 'Entități',
      icon: 'pi pi-users',
      routerLink: '/entitati'
    },
    {
      label: 'Registre',
      icon: 'pi pi-book',
      routerLink: '/registre'
    },
    {
      label: 'Administrare',
      icon: 'pi pi-cog',
      routerLink: '/admin',
      visible: this.auth.hasAnyRole(['superadmin', 'institution_admin'])
    }
  ];

  userInitials(): string {
    const info = this.auth.userInfo();
    if (!info) return '?';
    if (info.given_name && info.family_name) {
      return (info.given_name[0] + info.family_name[0]).toUpperCase();
    }
    return info.email[0].toUpperCase();
  }
}
```

- [ ] **Step 17.3: Commit**

```bash
git add frontend/src/app/app.component.ts frontend/src/app/core/
git commit -m "feat: add navbar and app root with routing-based layout"
```

---

## Task 18: Dashboard component

**Files:**
- Create: `frontend/src/app/dashboard/dashboard.component.ts`

- [ ] **Step 18.1: Write dashboard.component.ts**

```typescript
// frontend/src/app/dashboard/dashboard.component.ts
import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { CardModule } from 'primeng/card';
import { ButtonModule } from 'primeng/button';
import { TagModule } from 'primeng/tag';
import { TableModule } from 'primeng/table';
import { SkeletonModule } from 'primeng/skeleton';
import { AuthService } from '../core/services/auth.service';
import { RegistraturaService } from '../core/services/registratura.service';
import { Document } from '../core/models/document.model';

@Component({
  selector: 'app-dashboard',
  standalone: true,
  imports: [
    CommonModule, RouterLink,
    CardModule, ButtonModule, TagModule, TableModule, SkeletonModule
  ],
  template: `
    <div class="flex flex-col gap-6 p-6">
      <!-- Header -->
      <div class="flex items-center justify-between">
        <div class="flex flex-col gap-1">
          <h1 class="m-0 text-2xl font-bold">Bun venit, {{ auth.userInfo()?.given_name ?? auth.userInfo()?.email }}</h1>
          <span class="text-sm" style="color: var(--text-color-secondary)">Sistem de Gestiune Documente</span>
        </div>
        <p-button
          label="Document Nou"
          icon="pi pi-plus"
          routerLink="/registratura/nou"
        />
      </div>

      <!-- Stats row -->
      <div class="grid" style="grid-template-columns: repeat(4, 1fr); gap: 1.5rem;">
        <p-card>
          <div class="flex flex-col gap-2">
            <span style="color: var(--text-color-secondary)">Total Documente</span>
            @if (loading()) {
              <p-skeleton height="2rem" />
            } @else {
              <span class="text-3xl font-bold">{{ stats().total }}</span>
            }
          </div>
        </p-card>
        <p-card>
          <div class="flex flex-col gap-2">
            <span style="color: var(--text-color-secondary)">În Lucru</span>
            @if (loading()) {
              <p-skeleton height="2rem" />
            } @else {
              <span class="text-3xl font-bold">{{ stats().inLucru }}</span>
            }
          </div>
        </p-card>
        <p-card>
          <div class="flex flex-col gap-2">
            <span style="color: var(--text-color-secondary)">Asteaptă Aprobare</span>
            @if (loading()) {
              <p-skeleton height="2rem" />
            } @else {
              <span class="text-3xl font-bold">{{ stats().fluxAprobare }}</span>
            }
          </div>
        </p-card>
        <p-card>
          <div class="flex flex-col gap-2">
            <span style="color: var(--text-color-secondary)">Finalizate Azi</span>
            @if (loading()) {
              <p-skeleton height="2rem" />
            } @else {
              <span class="text-3xl font-bold">{{ stats().finalizateAzi }}</span>
            }
          </div>
        </p-card>
      </div>

      <!-- Recent documents -->
      <p-card header="Documente Recente">
        <p-table
          [value]="recentDocuments()"
          [loading]="loading()"
          [paginator]="false"
          styleClass="p-datatable-sm"
        >
          <ng-template pTemplate="header">
            <tr>
              <th>Nr. Înregistrare</th>
              <th>Tip</th>
              <th>Obiect</th>
              <th>Status</th>
              <th>Data</th>
              <th></th>
            </tr>
          </ng-template>
          <ng-template pTemplate="body" let-doc>
            <tr>
              <td><code>{{ doc.nr_inregistrare }}</code></td>
              <td>{{ doc.tip }}</td>
              <td class="max-w-xs" style="white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
                {{ doc.obiect }}
              </td>
              <td>
                <p-tag
                  [value]="statusLabel(doc.status)"
                  [severity]="statusSeverity(doc.status)"
                />
              </td>
              <td>{{ doc.data_inregistrare | date:'dd.MM.yyyy' }}</td>
              <td>
                <p-button
                  icon="pi pi-eye"
                  severity="secondary"
                  [text]="true"
                  [rounded]="true"
                  [routerLink]="['/registratura', doc.id]"
                />
              </td>
            </tr>
          </ng-template>
          <ng-template pTemplate="emptymessage">
            <tr>
              <td colspan="6" class="text-center p-6">
                <div class="flex flex-col items-center gap-3">
                  <i class="pi pi-inbox text-4xl" style="color: var(--text-color-secondary)"></i>
                  <span style="color: var(--text-color-secondary)">Nu există documente recente</span>
                  <p-button label="Înregistrează primul document" routerLink="/registratura/nou" />
                </div>
              </td>
            </tr>
          </ng-template>
        </p-table>
      </p-card>
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class DashboardComponent implements OnInit {
  auth = inject(AuthService);
  private registraturaService = inject(RegistraturaService);

  loading = signal(true);
  recentDocuments = signal<Document[]>([]);
  stats = signal({ total: 0, inLucru: 0, fluxAprobare: 0, finalizateAzi: 0 });

  ngOnInit(): void {
    this.loadDashboard();
  }

  private async loadDashboard(): Promise<void> {
    try {
      const result = await this.registraturaService.getDocuments({ limit: 10, page: 1 });
      this.recentDocuments.set(result.data);
      // Stats would come from a dedicated /api/stats endpoint
    } finally {
      this.loading.set(false);
    }
  }

  statusLabel(status: string): string {
    const labels: Record<string, string> = {
      INREGISTRAT: 'Înregistrat',
      ALOCAT_COMPARTIMENT: 'Alocat',
      IN_LUCRU: 'În Lucru',
      FLUX_APROBARE: 'Aprobare',
      FINALIZAT: 'Finalizat',
      ARHIVAT: 'Arhivat',
      ANULAT: 'Anulat',
    };
    return labels[status] ?? status;
  }

  statusSeverity(status: string): 'success' | 'info' | 'warn' | 'danger' | 'secondary' {
    const map: Record<string, 'success' | 'info' | 'warn' | 'danger' | 'secondary'> = {
      INREGISTRAT: 'info',
      ALOCAT_COMPARTIMENT: 'warn',
      IN_LUCRU: 'warn',
      FLUX_APROBARE: 'warn',
      FINALIZAT: 'success',
      ARHIVAT: 'secondary',
      ANULAT: 'danger',
    };
    return map[status] ?? 'secondary';
  }
}
```

- [ ] **Step 18.2: Commit**

```bash
git add frontend/src/app/dashboard/
git commit -m "feat: add dashboard with stats cards and recent documents table"
```

---

## Task 19: Document list and create form

**Files:**
- Create: `frontend/src/app/registratura/registratura.routes.ts`
- Create: `frontend/src/app/registratura/list/document-list.component.ts`
- Create: `frontend/src/app/registratura/create/document-create.component.ts`
- Create: `frontend/src/app/registratura/detail/document-detail.component.ts`
- Create: `frontend/src/app/core/services/registratura.service.ts`
- Create: `frontend/src/app/core/models/document.model.ts`

- [ ] **Step 19.1: Write document.model.ts**

```typescript
// frontend/src/app/core/models/document.model.ts
export type DocumentStatus =
  | 'INREGISTRAT'
  | 'ALOCAT_COMPARTIMENT'
  | 'IN_LUCRU'
  | 'FLUX_APROBARE'
  | 'FINALIZAT'
  | 'ARHIVAT'
  | 'ANULAT';

export type TipDocument =
  | 'INTRARE' | 'IESIRE' | 'INTERN' | 'PETITIE' | 'CONTRACT'
  | 'DECIZIE' | 'HOTARARE' | 'DISPOZITIE' | 'ADRESA' | 'NOTIFICARE'
  | 'RAPORT' | 'REFERAT' | 'ADEVERINTA' | 'CERTIFICAT' | 'AUTORIZATIE' | 'AVIZ';

export type Clasificare = 'PUBLIC' | 'INTERN' | 'CONFIDENTIAL' | 'SECRET';

export interface Document {
  id: string;
  nr_inregistrare: string;
  registru_id: string;
  institution_id: string;
  tip: TipDocument;
  status: DocumentStatus;
  clasificare: Clasificare;
  emitent_id?: string;
  destinatar_id?: string;
  obiect: string;
  continut?: string;
  cuvinte_cheie?: string[];
  data_inregistrare: string;
  data_document?: string;
  data_termen?: string;
  data_finalizare?: string;
  termen_pastrare_ani: number;
  archive_status: string;
  rejection_count: number;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CreateDocumentDTO {
  registru_id: string;
  tip: TipDocument;
  clasificare: Clasificare;
  emitent_id?: string;
  destinatar_id?: string;
  obiect: string;
  continut?: string;
  cuvinte_cheie?: string[];
  nr_file?: number;
  data_document?: string;
  data_termen?: string;
  nr_document_extern?: string;
}

export interface PaginatedDocuments {
  data: Document[];
  total: number;
  page: number;
  limit: number;
}
```

- [ ] **Step 19.2: Write registratura.service.ts**

```typescript
// frontend/src/app/core/services/registratura.service.ts
import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { firstValueFrom } from 'rxjs';
import { environment } from '../../../environments/environment';
import { Document, CreateDocumentDTO, PaginatedDocuments } from '../models/document.model';

export interface ListDocumentsParams {
  page?: number;
  limit?: number;
  status?: string;
  tip?: string;
  registru_id?: string;
  search?: string;
  data_de?: string;
  data_pana?: string;
}

@Injectable({ providedIn: 'root' })
export class RegistraturaService {
  private http = inject(HttpClient);
  private base = `${environment.apiUrl}/api/documents`;

  async getDocuments(params: ListDocumentsParams = {}): Promise<PaginatedDocuments> {
    let httpParams = new HttpParams();
    if (params.page) httpParams = httpParams.set('page', params.page);
    if (params.limit) httpParams = httpParams.set('limit', params.limit);
    if (params.status) httpParams = httpParams.set('status', params.status);
    if (params.tip) httpParams = httpParams.set('tip', params.tip);
    if (params.registru_id) httpParams = httpParams.set('registru_id', params.registru_id);
    if (params.search) httpParams = httpParams.set('search', params.search);
    if (params.data_de) httpParams = httpParams.set('data_de', params.data_de);
    if (params.data_pana) httpParams = httpParams.set('data_pana', params.data_pana);

    return firstValueFrom(this.http.get<PaginatedDocuments>(this.base, { params: httpParams }));
  }

  async getDocument(id: string): Promise<Document> {
    return firstValueFrom(this.http.get<Document>(`${this.base}/${id}`));
  }

  async createDocument(dto: CreateDocumentDTO): Promise<Document> {
    return firstValueFrom(this.http.post<Document>(this.base, dto));
  }

  async updateDocument(id: string, dto: Partial<CreateDocumentDTO>): Promise<Document> {
    return firstValueFrom(this.http.patch<Document>(`${this.base}/${id}`, dto));
  }

  async performWorkflowAction(documentId: string, action: string, payload: Record<string, any>): Promise<any> {
    return firstValueFrom(
      this.http.post(`${environment.apiUrl}/api/workflows/${documentId}/actions`, { action, ...payload })
    );
  }

  async uploadAttachment(documentId: string, file: File, description?: string): Promise<any> {
    const formData = new FormData();
    formData.append('file', file);
    if (description) formData.append('description', description);
    return firstValueFrom(this.http.post(`${this.base}/${documentId}/attachments`, formData));
  }
}
```

- [ ] **Step 19.3: Write registratura.routes.ts**

```typescript
// frontend/src/app/registratura/registratura.routes.ts
import { Routes } from '@angular/router';

export const REGISTRATURA_ROUTES: Routes = [
  {
    path: '',
    loadComponent: () => import('./list/document-list.component').then(m => m.DocumentListComponent)
  },
  {
    path: 'nou',
    loadComponent: () => import('./create/document-create.component').then(m => m.DocumentCreateComponent)
  },
  {
    path: ':id',
    loadComponent: () => import('./detail/document-detail.component').then(m => m.DocumentDetailComponent)
  }
];
```

- [ ] **Step 19.4: Write document-list.component.ts**

```typescript
// frontend/src/app/registratura/list/document-list.component.ts
import { Component, OnInit, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { FormsModule, ReactiveFormsModule, FormBuilder } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { TableModule } from 'primeng/table';
import { ButtonModule } from 'primeng/button';
import { TagModule } from 'primeng/tag';
import { InputTextModule } from 'primeng/inputtext';
import { DropdownModule } from 'primeng/dropdown';
import { CardModule } from 'primeng/card';
import { ToolbarModule } from 'primeng/toolbar';
import { CalendarModule } from 'primeng/calendar';
import { TooltipModule } from 'primeng/tooltip';
import { Document, DocumentStatus } from '../../core/models/document.model';
import { RegistraturaService, ListDocumentsParams } from '../../core/services/registratura.service';

@Component({
  selector: 'app-document-list',
  standalone: true,
  imports: [
    CommonModule, FormsModule, ReactiveFormsModule, RouterLink,
    TableModule, ButtonModule, TagModule, InputTextModule, DropdownModule,
    CardModule, ToolbarModule, CalendarModule, TooltipModule
  ],
  providers: [DatePipe],
  template: `
    <div class="flex flex-col gap-4 p-6">
      <!-- Page header -->
      <div class="flex items-center justify-between">
        <h1 class="m-0 text-2xl font-bold">Registratură</h1>
        <p-button label="Document Nou" icon="pi pi-plus" routerLink="nou" />
      </div>

      <!-- Filters -->
      <p-card>
        <div class="flex flex-wrap gap-3 items-end">
          <div class="flex flex-col gap-1">
            <label class="text-sm font-medium">Caută</label>
            <p-iconField iconPosition="left">
              <p-inputIcon styleClass="pi pi-search" />
              <input
                pInputText
                type="text"
                placeholder="Obiect, nr. înregistrare..."
                [(ngModel)]="searchQuery"
                (ngModelChange)="onSearchChange()"
                style="width: 280px;"
              />
            </p-iconField>
          </div>

          <div class="flex flex-col gap-1">
            <label class="text-sm font-medium">Status</label>
            <p-dropdown
              [options]="statusOptions"
              [(ngModel)]="selectedStatus"
              (ngModelChange)="loadDocuments()"
              placeholder="Toate statusurile"
              [showClear]="true"
              style="width: 180px;"
            />
          </div>

          <div class="flex flex-col gap-1">
            <label class="text-sm font-medium">Tip Document</label>
            <p-dropdown
              [options]="tipOptions"
              [(ngModel)]="selectedTip"
              (ngModelChange)="loadDocuments()"
              placeholder="Toate tipurile"
              [showClear]="true"
              style="width: 180px;"
            />
          </div>

          <p-button
            icon="pi pi-filter-slash"
            severity="secondary"
            label="Resetează"
            (onClick)="resetFilters()"
          />
        </div>
      </p-card>

      <!-- Document table -->
      <p-table
        [value]="documents()"
        [loading]="loading()"
        [lazy]="true"
        [paginator]="true"
        [rows]="pageSize"
        [totalRecords]="totalRecords()"
        [rowsPerPageOptions]="[10, 25, 50]"
        (onPage)="onPageChange($event)"
        styleClass="p-datatable-gridlines p-datatable-striped"
        [scrollable]="true"
        scrollHeight="calc(100vh - 380px)"
      >
        <ng-template pTemplate="header">
          <tr>
            <th style="width: 160px;">Nr. Înregistrare</th>
            <th style="width: 120px;">Tip</th>
            <th>Obiect</th>
            <th style="width: 130px;">Status</th>
            <th style="width: 110px;">Data</th>
            <th style="width: 110px;">Termen</th>
            <th style="width: 80px;"></th>
          </tr>
        </ng-template>

        <ng-template pTemplate="body" let-doc>
          <tr>
            <td>
              <code class="font-mono text-sm">{{ doc.nr_inregistrare }}</code>
            </td>
            <td>
              <p-tag [value]="doc.tip" severity="secondary" />
            </td>
            <td>
              <span [pTooltip]="doc.obiect" tooltipPosition="top">
                {{ doc.obiect | slice:0:60 }}{{ doc.obiect.length > 60 ? '...' : '' }}
              </span>
            </td>
            <td>
              <p-tag
                [value]="statusLabel(doc.status)"
                [severity]="statusSeverity(doc.status)"
              />
            </td>
            <td>{{ doc.data_inregistrare | date:'dd.MM.yyyy' }}</td>
            <td>
              @if (doc.data_termen) {
                <span [class]="isOverdue(doc.data_termen) ? 'text-red-600 font-semibold' : ''">
                  {{ doc.data_termen | date:'dd.MM.yyyy' }}
                </span>
              }
            </td>
            <td>
              <p-button
                icon="pi pi-eye"
                severity="secondary"
                [text]="true"
                [rounded]="true"
                [routerLink]="['/registratura', doc.id]"
                pTooltip="Vizualizează"
              />
            </td>
          </tr>
        </ng-template>

        <ng-template pTemplate="emptymessage">
          <tr>
            <td colspan="7">
              <div class="flex flex-col items-center gap-3 p-8">
                <i class="pi pi-inbox text-5xl" style="color: var(--text-color-secondary)"></i>
                <span style="color: var(--text-color-secondary)">Nu există documente cu filtrele aplicate</span>
                <p-button label="Înregistrează Document" routerLink="nou" />
              </div>
            </td>
          </tr>
        </ng-template>
      </p-table>
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class DocumentListComponent implements OnInit {
  private svc = inject(RegistraturaService);

  documents = signal<Document[]>([]);
  loading = signal(true);
  totalRecords = signal(0);

  page = 1;
  pageSize = 25;
  searchQuery = '';
  selectedStatus = '';
  selectedTip = '';
  private searchTimeout: any;

  statusOptions = [
    { label: 'Înregistrat', value: 'INREGISTRAT' },
    { label: 'Alocat Compartiment', value: 'ALOCAT_COMPARTIMENT' },
    { label: 'În Lucru', value: 'IN_LUCRU' },
    { label: 'Flux Aprobare', value: 'FLUX_APROBARE' },
    { label: 'Finalizat', value: 'FINALIZAT' },
    { label: 'Arhivat', value: 'ARHIVAT' },
    { label: 'Anulat', value: 'ANULAT' },
  ];

  tipOptions = [
    { label: 'Intrare', value: 'INTRARE' },
    { label: 'Ieșire', value: 'IESIRE' },
    { label: 'Intern', value: 'INTERN' },
    { label: 'Petiție', value: 'PETITIE' },
    { label: 'Contract', value: 'CONTRACT' },
    { label: 'Decizie', value: 'DECIZIE' },
    { label: 'Hotărâre', value: 'HOTARARE' },
    { label: 'Dispoziție', value: 'DISPOZITIE' },
    { label: 'Adresă', value: 'ADRESA' },
  ];

  ngOnInit(): void {
    this.loadDocuments();
  }

  async loadDocuments(): Promise<void> {
    this.loading.set(true);
    try {
      const params: ListDocumentsParams = {
        page: this.page,
        limit: this.pageSize,
      };
      if (this.selectedStatus) params.status = this.selectedStatus;
      if (this.selectedTip) params.tip = this.selectedTip;
      if (this.searchQuery) params.search = this.searchQuery;

      const result = await this.svc.getDocuments(params);
      this.documents.set(result.data);
      this.totalRecords.set(result.total);
    } catch (err) {
      console.error('Failed to load documents', err);
    } finally {
      this.loading.set(false);
    }
  }

  onSearchChange(): void {
    clearTimeout(this.searchTimeout);
    this.searchTimeout = setTimeout(() => this.loadDocuments(), 400);
  }

  onPageChange(event: any): void {
    this.page = Math.floor(event.first / event.rows) + 1;
    this.pageSize = event.rows;
    this.loadDocuments();
  }

  resetFilters(): void {
    this.searchQuery = '';
    this.selectedStatus = '';
    this.selectedTip = '';
    this.page = 1;
    this.loadDocuments();
  }

  isOverdue(dataTermen: string): boolean {
    return new Date(dataTermen) < new Date();
  }

  statusLabel(status: string): string {
    const labels: Record<string, string> = {
      INREGISTRAT: 'Înregistrat', ALOCAT_COMPARTIMENT: 'Alocat',
      IN_LUCRU: 'În Lucru', FLUX_APROBARE: 'Aprobare',
      FINALIZAT: 'Finalizat', ARHIVAT: 'Arhivat', ANULAT: 'Anulat',
    };
    return labels[status] ?? status;
  }

  statusSeverity(status: string): 'success' | 'info' | 'warn' | 'danger' | 'secondary' {
    const map: Record<string, 'success' | 'info' | 'warn' | 'danger' | 'secondary'> = {
      INREGISTRAT: 'info', ALOCAT_COMPARTIMENT: 'warn', IN_LUCRU: 'warn',
      FLUX_APROBARE: 'warn', FINALIZAT: 'success', ARHIVAT: 'secondary', ANULAT: 'danger',
    };
    return map[status] ?? 'secondary';
  }
}
```

- [ ] **Step 19.5: Write document-create.component.ts**

```typescript
// frontend/src/app/registratura/create/document-create.component.ts
import { Component, inject, signal, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { CardModule } from 'primeng/card';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { TextareaModule } from 'primeng/textarea';
import { DropdownModule } from 'primeng/dropdown';
import { CalendarModule } from 'primeng/calendar';
import { ChipsModule } from 'primeng/chips';
import { MessageModule } from 'primeng/message';
import { DividerModule } from 'primeng/divider';
import { RegistraturaService } from '../../core/services/registratura.service';
import { MessageService } from 'primeng/api';

@Component({
  selector: 'app-document-create',
  standalone: true,
  imports: [
    CommonModule, ReactiveFormsModule,
    CardModule, ButtonModule, InputTextModule, TextareaModule,
    DropdownModule, CalendarModule, ChipsModule, MessageModule, DividerModule
  ],
  template: `
    <div class="flex flex-col gap-6 p-6 max-w-4xl mx-auto">
      <div class="flex items-center gap-3">
        <p-button icon="pi pi-arrow-left" severity="secondary" [text]="true" routerLink="/registratura" />
        <h1 class="m-0 text-2xl font-bold">Înregistrare Document Nou</h1>
      </div>

      <form [formGroup]="form" (ngSubmit)="onSubmit()" class="flex flex-col gap-4">
        <!-- Type and registry -->
        <p-card header="Date de Bază">
          <div class="flex flex-wrap gap-4">
            <div class="flex flex-col gap-2" style="flex: 1; min-width: 200px;">
              <label class="font-medium" for="tip">Tip Document <span style="color: var(--red-500)">*</span></label>
              <p-dropdown
                id="tip"
                formControlName="tip"
                [options]="tipOptions"
                placeholder="Selectați tipul"
                [invalid]="isInvalid('tip')"
                class="w-full"
              />
              @if (isInvalid('tip')) {
                <p-message severity="error" text="Tipul documentului este obligatoriu" />
              }
            </div>

            <div class="flex flex-col gap-2" style="flex: 1; min-width: 200px;">
              <label class="font-medium" for="clasificare">Clasificare</label>
              <p-dropdown
                id="clasificare"
                formControlName="clasificare"
                [options]="clasificareOptions"
                placeholder="Public"
                class="w-full"
              />
            </div>
          </div>
        </p-card>

        <!-- Subject and content -->
        <p-card header="Conținut Document">
          <div class="flex flex-col gap-4">
            <div class="flex flex-col gap-2">
              <label class="font-medium" for="obiect">Obiect / Subiect <span style="color: var(--red-500)">*</span></label>
              <input
                id="obiect"
                type="text"
                pInputText
                formControlName="obiect"
                placeholder="Descriere scurtă a documentului"
                [invalid]="isInvalid('obiect')"
                class="w-full"
              />
              @if (isInvalid('obiect')) {
                <p-message severity="error" text="Obiectul documentului este obligatoriu" />
              }
            </div>

            <div class="flex flex-col gap-2">
              <label class="font-medium" for="continut">Conținut / Rezumat</label>
              <textarea
                id="continut"
                pTextarea
                formControlName="continut"
                rows="4"
                placeholder="Descriere detaliată..."
                class="w-full"
              ></textarea>
            </div>

            <div class="flex flex-col gap-2">
              <label class="font-medium" for="cuvinte_cheie">Cuvinte Cheie</label>
              <p-chips
                id="cuvinte_cheie"
                formControlName="cuvinte_cheie"
                placeholder="Adăugați etichete..."
                class="w-full"
              />
            </div>
          </div>
        </p-card>

        <!-- Dates and references -->
        <p-card header="Date și Referințe">
          <div class="flex flex-wrap gap-4">
            <div class="flex flex-col gap-2" style="flex: 1; min-width: 180px;">
              <label class="font-medium">Data Documentului</label>
              <p-calendar
                formControlName="data_document"
                dateFormat="dd.mm.yy"
                [showIcon]="true"
                placeholder="zi.lună.an"
                class="w-full"
              />
            </div>
            <div class="flex flex-col gap-2" style="flex: 1; min-width: 180px;">
              <label class="font-medium">Termen de Răspuns</label>
              <p-calendar
                formControlName="data_termen"
                dateFormat="dd.mm.yy"
                [showIcon]="true"
                placeholder="zi.lună.an"
                class="w-full"
              />
            </div>
            <div class="flex flex-col gap-2" style="flex: 1; min-width: 200px;">
              <label class="font-medium">Nr. Document Extern</label>
              <input
                type="text"
                pInputText
                formControlName="nr_document_extern"
                placeholder="Nr. atribuit de emitent"
                class="w-full"
              />
            </div>
          </div>
        </p-card>

        <!-- Actions -->
        <div class="flex justify-end gap-3">
          <p-button label="Anulează" severity="secondary" routerLink="/registratura" />
          <p-button
            type="submit"
            label="Înregistrează Document"
            icon="pi pi-check"
            [loading]="saving()"
            [disabled]="form.invalid"
          />
        </div>
      </form>
    </div>
  `,
  changeDetection: ChangeDetectionStrategy.OnPush
})
export class DocumentCreateComponent {
  private fb = inject(FormBuilder);
  private svc = inject(RegistraturaService);
  private router = inject(Router);
  private messageService = inject(MessageService);

  saving = signal(false);

  form = this.fb.group({
    tip: ['', Validators.required],
    clasificare: ['PUBLIC'],
    obiect: ['', [Validators.required, Validators.minLength(5), Validators.maxLength(500)]],
    continut: [''],
    cuvinte_cheie: [[]],
    data_document: [null as Date | null],
    data_termen: [null as Date | null],
    nr_document_extern: [''],
  });

  tipOptions = [
    { label: 'Intrare', value: 'INTRARE' },
    { label: 'Ieșire', value: 'IESIRE' },
    { label: 'Intern', value: 'INTERN' },
    { label: 'Petiție', value: 'PETITIE' },
    { label: 'Contract', value: 'CONTRACT' },
    { label: 'Decizie', value: 'DECIZIE' },
    { label: 'Hotărâre', value: 'HOTARARE' },
    { label: 'Dispoziție', value: 'DISPOZITIE' },
    { label: 'Adresă', value: 'ADRESA' },
    { label: 'Raport', value: 'RAPORT' },
    { label: 'Referat', value: 'REFERAT' },
    { label: 'Adeverință', value: 'ADEVERINTA' },
    { label: 'Certificat', value: 'CERTIFICAT' },
    { label: 'Autorizație', value: 'AUTORIZATIE' },
    { label: 'Aviz', value: 'AVIZ' },
  ];

  clasificareOptions = [
    { label: 'Public', value: 'PUBLIC' },
    { label: 'Intern', value: 'INTERN' },
    { label: 'Confidențial', value: 'CONFIDENTIAL' },
    { label: 'Secret', value: 'SECRET' },
  ];

  isInvalid(field: string): boolean {
    const control = this.form.get(field);
    return !!(control?.invalid && control?.touched);
  }

  async onSubmit(): Promise<void> {
    if (this.form.invalid) {
      this.form.markAllAsTouched();
      return;
    }
    this.saving.set(true);
    try {
      const val = this.form.value;
      const dto = {
        registru_id: '', // TODO: selected from dropdown in production
        tip: val.tip as any,
        clasificare: val.clasificare as any,
        obiect: val.obiect!,
        continut: val.continut || undefined,
        cuvinte_cheie: val.cuvinte_cheie || undefined,
        data_document: val.data_document?.toISOString() || undefined,
        data_termen: val.data_termen?.toISOString() || undefined,
        nr_document_extern: val.nr_document_extern || undefined,
      };
      const doc = await this.svc.createDocument(dto);
      this.messageService.add({
        severity: 'success',
        summary: 'Document înregistrat',
        detail: `Nr. ${doc.nr_inregistrare}`
      });
      this.router.navigate(['/registratura', doc.id]);
    } catch (err) {
      this.messageService.add({ severity: 'error', summary: 'Eroare', detail: 'Nu s-a putut înregistra documentul' });
    } finally {
      this.saving.set(false);
    }
  }
}
```

- [ ] **Step 19.6: Commit**

```bash
git add frontend/src/app/registratura/ frontend/src/app/core/
git commit -m "feat: add document list with filters, create form, service layer"
```

---

## Task 20: Build and verify frontend

- [ ] **Step 20.1: Run Angular build**

```bash
cd /c/dev/egudoc/frontend
ng build --configuration=development 2>&1 | tail -20
```

Expected: build succeeds with no errors (warnings about missing components are OK during early development).

- [ ] **Step 20.2: Commit final frontend state**

```bash
cd /c/dev/egudoc
git add frontend/
git commit -m "feat: complete frontend foundation - dashboard, registratura list/create, OIDC auth"
git push origin master
```

---

## Sub-plan C Completion Checklist

- [ ] `ng build` produces no errors
- [ ] OIDC auth flow works against eguilde (redirect → login → callback → token stored)
- [ ] Auth guard redirects unauthenticated users to eguilde login
- [ ] Navbar shows user email and role-based menu items
- [ ] Dashboard displays document stats and recent documents
- [ ] Document list shows paginated, filterable documents
- [ ] Document create form validates and submits
- [ ] All colors from PrimeNG theme only (no Tailwind color classes)
- [ ] All UI components from PrimeNG (no custom HTML buttons/inputs/tables)
- [ ] OnPush change detection on all components

---

*Next: Sub-plan D — QTSP Integration (eDelivery + eArchiving + PDF/A + E-ARK)*
