export const environment = {
  production: false,
  apiUrl: 'http://localhost:8090',
  oidc: {
    issuer: 'http://localhost:3100/api/oidc',
    clientId: 'egudoc-spa',
    redirectUri: 'http://localhost:4200/auth/callback',
    scope: 'openid profile email offline_access',
    responseType: 'code',
    requireHttps: false,
    showDebugInformation: true,
    usePkce: true,
  }
};
