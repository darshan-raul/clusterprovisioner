// ============================================================
// App configuration — loaded at build time via --dart-define
// ============================================================

const kCognitoUserPoolId  = String.fromEnvironment('COGNITO_POOL_ID', defaultValue: '');
const kCognitoAppClientId = String.fromEnvironment('COGNITO_CLIENT_ID', defaultValue: '');
const kApiBaseUrl         = String.fromEnvironment('API_BASE_URL', defaultValue: '');
const kAwsRegion          = String.fromEnvironment('AWS_REGION', defaultValue: 'ap-south-1');
