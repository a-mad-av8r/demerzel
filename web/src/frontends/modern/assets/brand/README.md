# Brand assets

- `web/public/assets/demerzel_logo.png` is the user-supplied full logo. Modern and classic headers and the modern sign-in page use it.
- `web/public/favicon.png` is the separate compact favicon. `web/index.html` links to `/favicon.png`; the server serves it as `image/png` without caching across updates.
- Keep these images under `web/public`, not `internal/webui/dist`: Vite regenerates `dist` before Go embeds it.

The public logo URL is served with immutable caching; update its version query in both UI implementations when replacing the image. The favicon version query in `web/index.html` follows the same rule.

`components/GitHubIcon.vue` uses GitHub's [Octicons mark-github-16](https://github.com/primer/octicons/blob/main/icons/mark-github-16.svg) with `currentColor`. Its MIT license remains in `octicons.LICENSE`.
