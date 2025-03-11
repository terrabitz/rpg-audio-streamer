import { defineConfig } from '@hey-api/openapi-ts';

export default defineConfig({
  input: '../openapi.yaml',
  output: 'src/apiClient',
  plugins: [{
    name: '@hey-api/client-fetch',
    runtimeConfigPath: './src/plugins/api.ts',
  }],
});