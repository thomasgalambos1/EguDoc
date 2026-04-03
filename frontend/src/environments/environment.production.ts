export const environment = {
  production: true,
  apiUrl: '/api',
  oidc: {
    issuer: 'https://eguilde.ro/api/oidc',
    clientId: 'egudoc-spa',
    redirectUri: 'https://egudoc.eguilde.ro/auth/callback',
    scope: 'openid profile email offline_access',
    responseType: 'code',
    requireHttps: true,
    showDebugInformation: false,
  }
};
