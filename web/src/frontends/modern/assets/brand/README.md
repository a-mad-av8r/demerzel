# 新版品牌资源

来自用户确认的 GPT-Load Coral Anchor 资源包，保留原始吉祥物轮廓和配色。Demerzel 字样由界面普通文本显示，不新增或重绘矢量 Logo：

- `icon.svg`：原 `web/icon.svg`，保留为方形吉祥物图标。
- `components/BrandLogo.vue`：复用原有吉祥物轮廓；收起时显示方形图标，展开时旁边以普通文本显示 Demerzel。

图形使用品牌橘红 `#FF4F1F`；文字颜色由现代主题 token 控制。浅/深色横版 Logo SVG 未被引用并已移除；界面配色仍由 `styles/tokens.css` 独立维护。

这些资源仅由 modern 引用。classic 的 `BrandMark.vue` 使用相同吉祥物路径，保持无文字、透明背景和原有方形尺寸，通过 `currentColor` 沿用旧版的 `--color-action`，适配浅色和深色主题。

网站图标 `web/public/favicon.svg` 使用同一吉祥物路径、品牌橘红填色和透明背景，等比放大以适应浏览器标签页的小尺寸。由 `web/index.html` 直接声明，新旧界面和登录页共用，不依赖前端启动后替换。图标 URL 带素材版本，更新素材时同步更新版本以刷新浏览器缓存。

`components/GitHubIcon.vue` 使用 GitHub 官方 [Octicons 的 mark-github-16](https://github.com/primer/octicons/blob/main/icons/mark-github-16.svg)，保留原始路径，以 `currentColor` 适配明暗主题。其 MIT 许可证见本目录的 `octicons.LICENSE`。
