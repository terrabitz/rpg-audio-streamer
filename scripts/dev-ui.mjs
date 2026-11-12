import { createRequire } from 'node:module'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

if (!process.send) throw new Error('Start the dev instance with make dev, not this worker directly')

const root = fileURLToPath(new URL('../ui/', import.meta.url))
const require = createRequire(new URL('../ui/package.json', import.meta.url))
const { createServer } = await import(require.resolve('vite'))
const [scratch, apiURL, port] = process.argv.slice(2)
const server = await createServer({
  root,
  envDir: scratch,
  cacheDir: join(scratch, 'vite-cache'),
  define: { 'import.meta.env.VITE_API_BASE_URL': JSON.stringify('/') },
  server: {
    host: '127.0.0.1',
    port: Number(port),
    strictPort: true,
    open: false,
    proxy: { '/api': { target: apiURL, ws: true, changeOrigin: true } },
  },
})
await server.listen()
process.send({ url: server.resolvedUrls.local[0] })
