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
