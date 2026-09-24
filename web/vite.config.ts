import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { compressStaticAsset } from './build/precompress.mjs'
import { readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'

export default defineConfig({
  plugins: [
    vue(),
    {
      name: 'precompress-static-assets',
      async writeBundle(options, bundle) {
        // Precompress at build time: the router only serves immutable bytes.
        // Read final files after Vite's post-processing and asset URL replacement.
        if (!options.dir) throw new Error('Static compression requires an output directory')
        for (const name of Object.keys(bundle)) {
          if (!/\.(js|css)$/.test(name)) continue
          const target = resolve(options.dir, name)
          const raw = await readFile(target)
          const compressed = compressStaticAsset(raw)
          if (compressed.length < raw.length) await writeFile(target + '.gz', compressed)
        }
      },
    },
  ],
  build: {
    outDir: '../internal/api/static',
    emptyOutDir: true,
  },
})
