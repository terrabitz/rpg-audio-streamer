import { defineConfig } from '@hey-api/openapi-ts';

export default defineConfig({
  input: '../openapi.yaml',
  output: 'src/apiClient',
  plugins: ['@hey-api/client-fetch'],
});