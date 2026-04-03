import { HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { AuthService } from '../services/auth.service';
import { environment } from '../../../environments/environment';

export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const auth = inject(AuthService);
  // In dev: apiUrl is absolute (http://localhost:8090), matches absolute URLs
  // In prod: apiUrl is '/api', matches relative paths — always use relative URLs in services
  if (!req.url.startsWith(environment.apiUrl)) {
    return next(req);
  }
  const token = auth.getAccessToken();
  if (!token) {
    return next(req);
  }
  return next(req.clone({ setHeaders: { Authorization: `Bearer ${token}` } }));
};
