npx skills add https://github.com/vercel-labs/agent-skills --skill vercel-react-best-practices -a claude-code
npx skills add https://github.com/anthropics/skills --skill frontend-design -a claude-code
npx skills add vercel-labs/agent-skills --skill web-design-guidelines -a claude-code
npx skills add https://github.com/jeffallan/claude-skills --skill golang-pro -a claude-code

ln -s .claude .agents
ln -s .claude .codex
ln -s .claude .opencode