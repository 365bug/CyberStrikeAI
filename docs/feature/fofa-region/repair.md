# FOFA 返回字段兼容修复

## F0 接入与工作流门禁

2026-10-16：本地 main a4f01730 工作区干净；专用分支 codex/fix-fofa-region。
用户在已说明同步调整前后端 province/region 和资产导入映射的核查结果后要求修复。
阶段 P10 反馈修复，进入实现及 QA；依赖为已有接口、页面、官方 FOFA fields 约定和核查报告，均已取得。无需新的 UX 或 API 设计决策，允许窄范围实现。

可维护性门禁：fofa.go 1202 行，info-collect.js 1749 行，assets.js 1842 行，index.html 7032 行；跨传输、配置和映射，风险高。采用 narrow_fix：大文件只修改默认值、调用点和字段映射，新增后端兼容逻辑放独立文件，不做广泛重构。已有 Go handler 模拟服务测试和 Node VM 前端测试可复用。

验收：默认字段符合 FOFA 约定；旧 province 请求转换且结果字段正确；资产导入保留省份；其他提供商不变。
