import { execFileSync, spawnSync } from 'node:child_process'
import { mkdir, readFile, readdir, rm } from 'node:fs/promises'
import path from 'node:path'

const manifestDir = path.resolve('test-results', 'recordings')
const outputDir = path.resolve(process.env.RECORDING_OUTPUT ?? 'recordings')
const fps = 10
const holdSeconds = 2
const background = '0x121212'

const run = spawnSync('npx', ['playwright', 'test', ...process.argv.slice(2)], {
  stdio: 'inherit',
  env: { ...process.env, E2E_RECORD: '1' },
})
await rm(outputDir, { recursive: true, force: true })
await mkdir(outputDir, { recursive: true })
const manifests = (await readdir(manifestDir).catch(() => [])).filter(name => name.endsWith('.json'))
for (const name of manifests) {
  const manifest = JSON.parse(await readFile(path.join(manifestDir, name), 'utf8'))
  if (manifest.status !== 'passed') {
    console.warn(`Skipping recording for ${manifest.title}: test ${manifest.status}`)
    continue
  }
  const gif = path.join(outputDir, name.replace(/\.json$/, '.gif'))
  renderGif(manifest, gif)
  console.log(`Recorded "${manifest.title}" -> ${path.relative(process.cwd(), gif)}`)
}
process.exit(run.status ?? 1)

function renderGif({ pages, durationMs }, gif) {
  const total = durationMs / 1000 + holdSeconds
  const [width, height] = pages.length <= 4 ? [960, 540] : [640, 360]
  const columns = pages.length === 1 ? 1 : pages.length <= 4 ? 2 : 3
  const cells = Math.ceil(pages.length / columns) * columns
  const filters = pages.map(({ offsetMs }, index) => [
    `[${index}:v]fps=${fps}`,
    `scale=${width}:${height}`,
    `tpad=start_duration=${offsetMs / 1000}:start_mode=add:color=${background}`,
    `tpad=stop_mode=clone:stop_duration=${total}`,
    `trim=duration=${total}`,
    `setpts=PTS-STARTPTS[v${index}]`,
  ].join(','))
  for (let index = pages.length; index < cells; index += 1) {
    filters.push(`color=c=${background}:s=${width}x${height}:r=${fps}:d=${total}[v${index}]`)
  }
  const tiles = Array.from({ length: cells }, (_, index) => `[v${index}]`).join('')
  const layout = Array.from({ length: cells }, (_, index) =>
    `${(index % columns) * width}_${Math.floor(index / columns) * height}`).join('|')
  filters.push(cells === 1
    ? `${tiles}null[grid]`
    : `${tiles}xstack=inputs=${cells}:layout=${layout}[grid]`)
  filters.push('[grid]split[a][b]', '[a]palettegen=stats_mode=diff[p]',
    '[b][p]paletteuse=dither=bayer:bayer_scale=5:diff_mode=rectangle')
  execFileSync('ffmpeg', [
    '-v', 'error', '-y',
    ...pages.flatMap(({ video }) => ['-i', video]),
    '-filter_complex', filters.join(';'),
    gif,
  ], { stdio: 'inherit' })
}
