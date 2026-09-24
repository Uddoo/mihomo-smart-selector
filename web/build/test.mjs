import { readdir } from 'node:fs/promises'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

const root = fileURLToPath(new URL('../', import.meta.url))

async function testFiles(directory) {
  const entries = await readdir(path.join(root, directory), { withFileTypes: true })
  const groups = await Promise.all(
    entries.map((entry) => {
      const relative = path.join(directory, entry.name)
      return entry.isDirectory()
        ? testFiles(relative)
        : entry.isFile() && entry.name.endsWith('.test.mjs')
          ? [relative]
          : []
    }),
  )
  return groups.flat()
}

// Discover only colocated unit tests, never the browser suite or dependencies.
const files = (await Promise.all(['src', 'build'].map(testFiles))).flat().sort()
if (!files.length) throw new Error('No unit tests found')
const result = spawnSync(process.execPath, ['--experimental-strip-types', '--test', ...files], {
  cwd: root,
  stdio: 'inherit',
  windowsHide: true,
})
if (result.error) throw result.error
process.exitCode = result.status ?? 1
