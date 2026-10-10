# 调用拦截配置修改冲突检测

## F0 Intake / Gates

2026-10-10，用户要求修复已核实的多用户规则覆盖问题，并明确目标为当前开源版 CyberStrikeAI。main 起点 a8321c4b，工作目录干净；分支 codex/tool-guard-conflict-detection。Pro 版由其他聊天处理，本分支不编辑其文件。

Workflow Gate：P10 缺陷反馈 → P7 局部修复 → P9 验证。上游为用户问题、截图、已完成的源码核实；允许实现保存冲突检测，验收同版本只有一次成功，失败保留草稿并提示刷新。

Maintainability Gate：config.go 2940 行、tool_guard.go 175 行、tool-guard.js 732 行。风险高，按 narrow_fix 执行；config.go 只加一个受现有锁保护的版本字段，行为放在现有专项 handler；不新增全局配置职责，不做无关重构。前后端接口联动，需 handler race 测试和现有前端测试。

Lifecycle Report：F0–F6；影响 handler、调用拦截 JS 和文档；权限和业务规则保持原有约束。GET/PUT 增加版本头，旧无条件 PUT 将拒绝，这是用户要求的覆盖保护所必需；客户端须先读取版本。阶段文档使用本文件，逐阶段提交推送。未要求合并、发布或部署。
