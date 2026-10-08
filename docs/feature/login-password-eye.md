
## F0 Intake

用户在确认内置眼睛按钮方案后要求“最佳实践优化”。仓库 main 干净，起点 762f798。独立分支 codex/login-password-eye。
Workflow Gate: P7 局部 UI 修整→P9 验证；上游为用户截图及已确认方案；无 API/权限变化，允许实现。
Maintainability Gate: auth.js 1014 行、模板 6987 行、style.css 46717 行。大文件风险高，但只替换现有组件的局部标记/样式/标签同步，不新增职责，按 narrow_fix 执行；无需整体重构。

## F1 Requirements

范围依据：用户明确要求执行上一轮方案。无需另行产品决策。
REQ-001 / AC-001: 密码框与用户名框同宽，右侧内置灰色线条眼睛，图标 20px、触控区域 44px。
REQ-002 / AC-002: 原生非提交按钮支持键盘 Enter/Space，焦点可见；显示/隐藏切换不改变密码值。
REQ-003 / AC-003: 图标随状态切换，显示密码/隐藏密码标签和悬浮提示随现有语言系统同步；登录框重新打开仍默认隐藏。
非目标：后端登录、密码策略及其他密码输入框。

## F2 Design

保留原按钮 ID 和 toggleLoginPasswordVisibility 入口；SVG 为装饰图像并 aria-hidden。按钮以 aria-label/title 命名，以现有 aria-pressed 表示状态；data-i18n-skip-text 防止翻译覆盖图标，data-i18n-attr 同步标签。
相对定位容器+绝对定位按钮；输入框预留右内边距，图标呈现由 aria-pressed 的 CSS 选择器控制。使用现有主题色、原生按钮交互，不引入依赖。

## F3 Plan

修改 index.html 的按钮内部结构，style.css 的局部布局和交互样式，auth.js 的标签同步，扩充已有 password-feedback.test.cjs 的状态/标签断言。
验证：已有密码反馈测试、JS syntax、diff check，实际浏览器桌面/窄屏与语言、键盘切换检查。回退可 revert 本功能代码提交。发布记录在 CHANGELOG.md Unreleased；不合并或发布。
