# FOFA 返回字段兼容修复

## F0 接入与工作流门禁

2026-10-16：本地 main a4f01730 工作区干净；专用分支 codex/fix-fofa-region。
用户在已说明同步调整前后端 province/region 和资产导入映射的核查结果后要求修复。
阶段 P10 反馈修复，进入实现及 QA；依赖为已有接口、页面、官方 FOFA fields 约定和核查报告，均已取得。无需新的 UX 或 API 设计决策，允许窄范围实现。

可维护性门禁：fofa.go 1202 行，info-collect.js 1749 行，assets.js 1842 行，index.html 7032 行；跨传输、配置和映射，风险高。采用 narrow_fix：大文件只修改默认值、调用点和字段映射，新增后端兼容逻辑放独立文件，不做广泛重构。已有 Go handler 模拟服务测试和 Node VM 前端测试可复用。

验收：默认字段符合 FOFA 约定；旧 province 请求转换且结果字段正确；资产导入保留省份；其他提供商不变。

## F1 修复范围

状态：用户已授权实施上述核查结论对应的修复（2026-10-16，“帮我修复？”）；本节记录该既有授权范围，不引入新的产品需求。
REQ-001 / AC-001：默认 FOFA 信息收集只请求受支持返回字段，省略 fields 的直接调用同样有效。
REQ-002 / AC-002：旧表单或 API 请求包含 province 时可继续查询，字段与行值顺序一致。
REQ-003 / AC-003：返回的 region 可导入资产 province，兼容已有 province 和 Quake 省份结果。
不改变查询语法、数据库结构、其他提供商字段、错误状态处理；不发布或部署。无待用户决策。

## F2 接口与数据流

沿用 POST /api/fofa/search。仅 FOFA 将返回字段别名 province 转成 region；去除重复字段，保持首次出现顺序；请求和响应 fields 使用同一规范化列表，因此结果映射不偏移。其他提供商字段不变。
前端和 HTML 默认及预设使用 region；FOFA 情报预设的 as_number/as_organization 同属错误返回字段，改用官方 asn/org。工具说明同步更新。资产导入优先 region，再兼容 province、location.province_cn。
兼容转换只针对返回字段，不修改查询表达式。参考 https://fofa.info/api。

## F3 实施计划

新增独立 Go 字段规范化辅助文件；handler 加调用点，修改默认值；前端配置、HTML 和工具说明定点替换；资产映射加 region。
验证：Go 模拟服务检查真实 outbound fields 并返回按序行，覆盖默认、旧别名、重复和结果映射；Node 验证 FOFA 预设以及对象/数组资产导入，保留旧数据及 Quake 行为。执行现有 handler、前端相关测试和语法、diff 检查。
回退：撤销本分支代码提交，无数据迁移。真实 API 查询会消耗外部账户额度，本次采用合同模拟验证，不使用真实凭据。

## F4 实施记录

已修正后端、JS、HTML 默认和 FOFA 字段预设；独立 normalizeFofaFields 在 FOFA 分支转换 province、清理重复字段并保持请求/响应顺序。空白 fields 同空值使用默认值。旧表单可由后端兼容，无需清空浏览器存储；保留旧 FOFA 默认字段识别，避免其他提供商恢复表单时误用 FOFA 字段。
资产导入读取 region，并兼容 province、location.province_cn；工具字段说明同步更新为 region/asn/org。
已通过 FOFA 和其他提供商定向 Go handler 测试、16 项 Node 测试、JS 语法及 diff 检查。新增测试最初因 Node VM 提取函数的结束标记拼写错误失败，修正测试加载边界后通过；未掩盖运行错误。完整 handler 包测试另行记录于 F5。

## F5 验证结果

2026-10-16：
- `go test ./internal/handler -run 'Test(Fofa|SafeFofa|.*SpaceSearch|.*ZoomEye|.*Quake|.*Shodan)' -count=1`：通过。
- `go test ./internal/handler -count=1`：完整包通过（16.683s）。
- `node --test web/static/js/fofa-fields.test.cjs web/static/js/asset-export-language.test.cjs web/static/js/asset-scan-prompt.test.cjs`：16/16 通过。
- `node --check web/static/js/info-collect.js`、`node --check web/static/js/assets.js`、`git diff --check`：通过。
新增 Go 合同测试验证实际发往模拟上游的字段、按序返回值和最终 JSON 映射，覆盖默认、空白、旧表单、别名重复及规范字段；Node 测试执行实际配置和导入函数，覆盖 HTML 默认、所有 FOFA 字段预设、对象/数组结果及旧字段和 Quake 兼容。
未调用真实 FOFA 账户、未执行浏览器人工查询或线上验证；这些不属于本次自动化验证结论。
