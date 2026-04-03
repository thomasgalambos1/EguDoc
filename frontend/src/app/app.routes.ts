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
