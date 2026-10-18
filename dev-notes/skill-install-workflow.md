# ZCode 手动安装第三方技能

> 从 ZCode 本地记忆导出（2026-10-18 重装电脑前备份）。

从 GitHub 仓库（如 MiniMax-AI/MiniMax-H3）安装第三方 agent 技能到 ZCode 的方法。`npx skills add`（Vercel skills CLI）支持 Claude Code/Cursor/Codex 等约 20 个 agent，但**不认识 ZCode**，直接运行对 ZCode 无效。

**原理**：ZCode 按目录约定发现技能（用户级 `~/.zcode/skills/`、`~/.agents/skills/`；工作区级 `<repo>/.zcode/skills/`、`.agents/skills/`）。手动放置含 `SKILL.md` 的目录即可；技能在**会话启动时**扫描，装完需新开会话才生效。

**步骤**：
1. curl 从 `raw.githubusercontent.com/<owner>/<repo>/main/<skill-dir>/` 下载（SKILL.md + references 等子目录，保持原始字节；GitHub API 匿名调用易 403 限流，机器上的 `gh` 若是同名 npm 包则不是官方 CLI，认证不可用）
2. 装到用户级 `~/.zcode/skills/<name>/`
3. 装前审查 SKILL.md 内容与 frontmatter（name/description）

**已装**（2026-09-03）：`~/.zcode/skills/h3-prompt-writing/`（MiniMax H3 视频生成提示词写作指南，SKILL.md + references + agents/openai.yaml）。
