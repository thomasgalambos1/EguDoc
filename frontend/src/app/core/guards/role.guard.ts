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
    router.navigate(['/dashboard'], { queryParams: { error: 'access_denied' } });
    return false;
  };
}
