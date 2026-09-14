# Project branding

## GitHub sharing

- `social-preview.png`: 1280 × 640 PNG, below 1 MiB, ready for GitHub's Social
  preview setting. It uses the existing logo and a real UI crop with mock data.
- `social-preview.html`: editable source for the cover, rendered with Playwright.
  Relative image references keep the source portable inside the repository.
- Regeneration and screenshot provenance: [showcase assets](../screenshots/README.md).

Repository description (applied and read back on 2026-09-14):

> Compare Mihomo / OpenClash nodes by service. Retest candidates, compare with your current node, and review switch outcomes. 按服务复测与对比节点，让切换有依据。

Chinese introduction:

> 给现有 Mihomo / OpenClash 加一个节点对比工作台：按服务测试，复测候选、对比当前节点，再确认切换。

About website: [Download / Releases](https://github.com/Uddoo/mihomo-smart-selector/releases).
This leads directly to published downloads while the project has no separate website.

Topics (existing tags retained, with `clash-party` and `network-monitoring` added):

```text
clash clash-party golang istoreos latency mihomo network-monitoring
network-tools openclash openwrt proxy self-hosted vue
```

The custom Social preview was confirmed enabled through GitHub's
`usesCustomOpenGraphImage` field on 2026-09-14. Keep the configured image;
`social-preview.png` remains the reusable sharing asset and the original
full-width header image in both READMEs, followed by a dated real screenshot.

`README.md` is the Chinese repository landing page. `README.en.md` contains the
English version; `README.zh-CN.md` preserves the old Chinese links as a short
compatibility entry point. Keep the original header artwork in both full versions.

The description, website and Topics above were applied through GitHub and
verified by API readback on 2026-09-14. Future edits to this file do not update
those settings automatically; GitHub metadata is maintained separately.

## Existing identity

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
