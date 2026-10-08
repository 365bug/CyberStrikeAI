
## F0 Intake

用户在确认内置眼睛按钮方案后要求“最佳实践优化”。仓库 main 干净，起点 762f798。独立分支 codex/login-password-eye。
Workflow Gate: P7 局部 UI 修整→P9 验证；上游为用户截图及已确认方案；无 API/权限变化，允许实现。
Maintainability Gate: auth.js 1014 行、模板 6987 行、style.css 46717 行。大文件风险高，但只替换现有组件的局部标记/样式/标签同步，不新增职责，按 narrow_fix 执行；无需整体重构。
