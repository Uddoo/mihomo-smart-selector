# Project branding

- `logo.png`: the user's selected fourth candidate, copied unchanged for both READMEs (1254 × 1254).
- `favicon-source.png`: the simplified radar, pointer, and selected-node artwork, edited with built-in image_gen (1254 × 1254).
- `../../../web/public/favicon-32.png`: 32 × 32 browser PNG, exported from the simplified source.
- `../../../web/public/favicon.ico`: PNG-compressed 16 × 16, 32 × 32, and 48 × 48 frames exported from the same source.

The favicon exports use Windows System.Drawing high-quality bicubic downsampling.
The source images remain at their original resolution. Vite copies `web/public` into
`internal/api/static` during `pnpm --dir web build`, so the Go binary serves the icons.

The complex README artwork retains the original server and radar details; the browser
version reduces these to one ring, a pointer, two candidate dots, and one green target.

## Exact favicon edit prompt

Provider: built-in image_gen. Reference: the selected original saved as `logo.png`.

```text
Use the attached image as the edit target and design reference. Create a radically simplified browser favicon version of this exact radar-and-node concept, designed to be legible at 16 and 32 pixels. One square image, centered compact symbol. Keep the original deep navy, electric cyan-blue and mint-green identity, and the pointer aimed toward the selected upper-right green node.
Composition: solid full-bleed dark navy background #081B34. A single thick clean cyan-blue circular arc around a central pointer; just TWO small blue candidate dots at the left and lower-left, and ONE clearly larger mint-green selected dot at approximately 1:30 on the arc. A strong simple filled pale-cyan triangular compass pointer starts near the center and points diagonally up-right toward that green dot. Leave a visible dark gap between the pointer tip and the green dot, so the two silhouettes stay distinct. The pointer should be broad and easily recognizable, not a tiny needle. Keep the green node large and clearly separated from the blue ring.
Remove the server hardware at the bottom, ALL waves, ALL radar grid rings, ALL tick marks, ALL dashed lines, ALL peripheral decorative dots, ALL 3D bevels, ALL gloss, ALL shadows, ALL glow, ALL texture, and the outer rounded tile frame. No gradients. Every shape uses one flat solid color, pristine vector-like geometry and rounded ends on the arc. No text, no letters, no wordmark, no watermarks, no extra symbols.
The composition should occupy approximately 85 percent of the square, with modest even edge clearance. Produce one finished simplified square image, not a presentation sheet or multiple-size layout.
```
