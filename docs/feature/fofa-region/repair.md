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
