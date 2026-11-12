import { defineConfig } from '@hey-api/openapi-ts';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  input: '../openapi.yaml',
  output: 'src/client/apiClient',
  plugins: [{
    name: '@hey-api/client-fetch',
    runtimeConfigPath: fileURLToPath(new URL('./src/plugins/api.ts', import.meta.url)),
  }],
});