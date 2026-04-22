export const environment = {
  production: true,
  apiUrl: '/api',
  oidc: {
    issuer: 'https://portal.eguilde.cloud/api/oidc',
    clientId: 'egudoc-spa',
    redirectUri: window.location.origin + '/auth/callback',
    postLogoutRedirectUri: window.location.origin,
    scope: 'openid profile email offline_access',
    responseType: 'code',
    usePkce: true,
    requireHttps: true,
    showDebugInformation: false,
  }
};
