# Geist fonts

Self-hosted variable WOFF2 files from [vercel/geist-font](https://github.com/vercel/geist-font/tree/10dc7658f13c38a474cde201bb09a4617267545b), pinned to commit `10dc7658f13c38a474cde201bb09a4617267545b`.

- `Geist.woff2`: `fonts/Geist/webfonts/Geist[wght].woff2`
- `GeistMono.woff2`: `fonts/GeistMono/webfonts/GeistMono[wght].woff2`
- Both use the SIL Open Font License; the upstream license is preserved in `OFL.txt`.
- `web/public/licenses/Geist-OFL.txt` packages that license with the embedded application.

Latin text uses Geist; Chinese text uses the installed system CJK font. Geist Mono is reserved for code and numeric evidence. Vite packages the fonts with the application, so browsers do not contact a font CDN.
