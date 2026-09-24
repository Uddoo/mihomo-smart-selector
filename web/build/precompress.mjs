import { gzipSync } from 'fflate'

// Pin the compressor implementation and timestamp so Node/zlib versions and
// the host OS cannot change the checked-in gzip representation.
export function compressStaticAsset(bytes) {
  return gzipSync(bytes, { level: 9, mtime: 0 })
}
