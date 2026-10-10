# REST 删除错误与空列表修复

## F0：接入与门禁

- 分支：codex/rest-error-empty-results；起始提交 68df7c1，工作区干净。
- P10 反馈修复，用户已明确授权修改及必要验证。已有只读分析：工作区 reviews/P-022-rest-semantics/analysis.md。
- 涉及 handler 的 c2.go (1267 行)、webshell.go (1097 行)、vulnerability.go (632 行)、batch_task_manager.go (1486 行)。较大文件负责模块 HTTP 或队列业务；已有模块测试，缺少本次 HTTP 合同回归覆盖。
- Maintainability Gate：高风险文件采用 narrow_fix，只改错误分支及空切片初始化，不增加职责、不移动代码；无需先重构。
- Workflow Gate：上游接口及实现已核对，期望明确，无阻塞依赖；允许实施。验证使用临时 SQLite 和 httptest，不访问部署服务。
- 生命周期范围：完成修复、验证、文档及变更记录；不合并、不部署。
