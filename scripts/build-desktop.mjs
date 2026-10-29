import { spawnSync } from 'node:child_process'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.resolve(scriptDir, '..')
const projectDir = path.join(repoRoot, 'cmd', 'desktop')
const useGoRun = process.argv.includes('--go-run')
const args = useGoRun
  ? ['run', 'github.com/wailsapp/wails/v2/cmd/wails', 'build', '-m', '-nosyncgomod']
  : ['build', '-m', '-nosyncgomod']

if (process.platform === 'win32') args.push('-webview2', 'browser')
if (process.env.DESKTOP_VERSION) args.push('-ldflags', `-X main.version=${process.env.DESKTOP_VERSION}`)

const result = spawnSync(useGoRun ? 'go' : 'wails', args, {
  cwd: projectDir,
  stdio: 'inherit',
  shell: process.platform === 'win32' && !useGoRun,
})

if (result.error) throw result.error
process.exit(result.status ?? 1)
