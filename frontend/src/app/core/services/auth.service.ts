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
      disablePKCE: !environment.oidc.usePkce,
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
      const info = await this.oauth.loadUserProfile() as Record<string, unknown>;
      this._userInfo.set({
        sub: info['sub'] as string,
        email: info['email'] as string,
        given_name: info['given_name'] as string | undefined,
        family_name: info['family_name'] as string | undefined,
        roles: (info['roles'] ?? []) as string[],
      });
    } catch (err) {
      console.error('Failed to load user info', err);
    }
  }
}
