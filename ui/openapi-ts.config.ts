import { defineConfig } from '@hey-api/openapi-ts';

export default defineConfig({
  input: '../openapi.yaml',
  output: 'src/client/apiClient',
  plugins: [{
    name: '@hey-api/client-fetch',
    runtimeConfigPath: '../../plugins/api.ts',
  }],
});