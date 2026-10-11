const fs = require('node:fs');
const vm = require('node:vm');
const test = require('node:test');
const assert = require('node:assert/strict');
const source = fs.readFileSync('web/static/js/dashboard.js', 'utf8');
const harness = () => {
    const context = { document: {}, dashboardState: { tokenDays: 7 }, refreshDashboard() {} };
    vm.createContext(context);
    vm.runInContext(source.slice(source.indexOf('function openDashboardTokenUsage()')), context);
    return context;
};

test('cache and reasoning details do not inflate total or double-count input', () => {
    const ctx = harness();
    const metrics = ctx.dashboardUsageMetrics({ totalTokens: 120, promptTokens: 100, completionTokens: 20, cachedTokens: 40, cacheWriteTokens: 30, reasoningTokens: 10, modelCalls: 1 });
    const values = Object.fromEntries(metrics);
    assert.equal(values.usageTotal, 120);
    assert.equal(values.usageCacheWrite, 30);
    assert.equal(values.usageCacheRate, '40.0%');
    assert.equal(Object.fromEntries(ctx.dashboardUsageMetrics({})).usageCacheRate, '—');
    assert.equal(ctx.dashboardUsageCount(Infinity), 0);
    assert.equal(ctx.dashboardUsageCount(-12), 0);
});

test('period selection refreshes once and rejects arbitrary periods', () => {
    const ctx = harness();
    let calls = 0;
    ctx.refreshDashboard = () => { calls++; };
    ctx.document.getElementById = () => ({ textContent: '' });
    ctx.changeDashboardTokenPeriod('30');
    ctx.changeDashboardTokenPeriod('999');
    assert.equal(ctx.dashboardState.tokenDays, 30);
    assert.equal(calls, 1);
});

test('all supported languages expose usage labels and interpolation placeholders', () => {
    for (const locale of ['zh-CN', 'en-US', 'ru-RU']) {
        const data = JSON.parse(fs.readFileSync(`web/static/i18n/${locale}.json`, 'utf8'));
        for (const key of ['usageView', 'usageScope', 'usageCacheRead', 'usageCacheWrite', 'usageTrend']) assert.ok(data.dashboard[key]);
        assert.match(data.dashboard.tokenUsageSub, /\{\{days\}\}/);
        assert.match(data.chat.turnTokenUsageTitle, /\{\{cacheWrite\}\}/);
    }
});
