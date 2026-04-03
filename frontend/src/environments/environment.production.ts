export const environment = {
  production: true,
  apiUrl: '/api',
  oidc: {
    issuer: 'https://eguilde.example.com/api/oidc',
    clientId: 'egudoc-spa',
    redirectUri: 'https://egudoc.example.com/auth/callback',
    scope: 'openid profile email offline_access',
    responseType: 'code',
    requireHttps: true,
    showDebugInformation: false,
    usePkce: true,
  }
};
