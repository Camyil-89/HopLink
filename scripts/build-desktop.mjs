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
if ((result.status ?? 1) !== 0) process.exit(result.status ?? 1)

// The executable is rebuilt at the same path, so ask Explorer to reload its
// icon instead of continuing to show a cached icon from the previous build.
if (process.platform === 'win32') {
  const executable = path.join(repoRoot, 'deploy', 'HopLink.exe')
  const quotePowerShell = value => `'${value.replaceAll("'", "''")}'`
  const signature = 'using System; using System.Runtime.InteropServices; public static class HopLinkShellNotify { [DllImport("shell32.dll", CharSet=CharSet.Unicode)] public static extern void SHChangeNotify(uint eventId, uint flags, string path, IntPtr item2); }'
  const command = `Add-Type -TypeDefinition '${signature}'; [HopLinkShellNotify]::SHChangeNotify(0x2000, 0x0005, ${quotePowerShell(executable)}, [IntPtr]::Zero); [HopLinkShellNotify]::SHChangeNotify(0x1000, 0x0005, ${quotePowerShell(path.dirname(executable))}, [IntPtr]::Zero)`
  const refresh = spawnSync('powershell.exe', ['-NoProfile', '-Command', command], { stdio: 'ignore' })
  if (refresh.error || refresh.status !== 0) {
    console.warn('Could not notify Explorer to refresh the HopLink icon.')
  }
}

process.exit(0)
