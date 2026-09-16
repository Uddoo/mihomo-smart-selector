// A small, same-origin blocking script applies the saved appearance before paint.
// Keep the allowlist aligned with src/appearance.ts; no credentials are read here.
(function () {
  function read(key) { try { return localStorage.getItem(key) } catch (_) { return null } }
  var palette = read('mss-palette')
  var mode = read('mss-theme')
  var root = document.documentElement
  root.dataset.palette = ['geist', 'ocean', 'iris', 'nord', 'catppuccin'].includes(palette) ? palette : 'geist'
  root.dataset.theme = mode === 'light' || mode === 'dark' ? mode : (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
})()
