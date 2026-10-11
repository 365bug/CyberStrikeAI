// Token detail data belongs to the settings section; the dashboard keeps a
// separate fixed seven-day summary.
var tokenUsageState = { days: 7, controller: null, requestSequence: 0, result: null };

function tokenUsageText(key, opts, fallback) {
    if (typeof window.t === 'function') {
        const text = window.t(key, opts);
        if (text && text !== key) return text;
    }
    return fallback == null ? key : fallback;
}

function openDashboardTokenUsage() {
    if (window.location.hash === '#settings?section=token-usage') {
        switchPage('settings');
        switchSettingsSection('token-usage');
    } else {
        window.location.hash = 'settings?section=token-usage';
    }
}

function stopTokenUsageRequests() {
    tokenUsageState.requestSequence++;
    if (tokenUsageState.controller) tokenUsageState.controller.abort();
    tokenUsageState.controller = null;
}

async function refreshTokenUsage() {
    const host = document.getElementById('settings-token-analysis');
    if (!host) return;
    stopTokenUsageRequests();
    const sequence = tokenUsageState.requestSequence;
    const controller = new AbortController();
    tokenUsageState.controller = controller;
    host.setAttribute('aria-busy', 'true');
    host.textContent = tokenUsageText('common.loading', null, '加载中…');
    const select = document.getElementById('settings-token-days');
    if (select) select.value = String(tokenUsageState.days);
    let url = '/api/usage/tokens?days=' + tokenUsageState.days + '&limit=20';
    const projectId = typeof getActiveProjectId === 'function' ? getActiveProjectId() : '';
    if (projectId) url += '&project_id=' + encodeURIComponent(projectId);
    try {
        const response = await apiFetch(url, { signal: controller.signal });
        const result = response && response.ok ? await response.json() : null;
        if (sequence !== tokenUsageState.requestSequence) return;
        tokenUsageState.result = result;
        renderTokenUsageAnalysis(result);
    } catch (_) {
        if (sequence !== tokenUsageState.requestSequence) return;
        tokenUsageState.result = null;
        renderTokenUsageAnalysis(null);
    } finally {
        if (sequence === tokenUsageState.requestSequence) {
            tokenUsageState.controller = null;
            host.removeAttribute('aria-busy');
        }
    }
}

function changeTokenUsagePeriod(value) {
    const days = Number(value);
    if (![7, 30, 90].includes(days)) return;
    tokenUsageState.days = days;
    refreshTokenUsage();
}

function tokenUsageCacheRate(summary) {
    const input = tokenUsageCount(summary && summary.promptTokens);
    const cached = tokenUsageCount(summary && summary.cachedTokens);
    return input > 0 ? (Math.min(cached, input) / input * 100).toFixed(1) + '%' : '—';
}

function tokenUsageCount(value) {
    const n = Number(value);
    return Number.isFinite(n) && n > 0 ? Math.trunc(n) : 0;
}

function tokenUsageMetrics(summary) {
    const input = tokenUsageCount(summary.promptTokens);
    const cached = tokenUsageCount(summary.cachedTokens);
    return [
        ['usageTotal', summary.totalTokens],
        ['usageInput', input],
        ['usageOutput', summary.completionTokens],
        ['usageCacheRead', cached],
        ['usageCacheWrite', summary.cacheWriteTokens],
        ['usageReasoning', summary.reasoningTokens],
        ['usageCalls', summary.modelCalls],
        ['usageCacheRate', tokenUsageCacheRate(summary)]
    ];
}

document.addEventListener('languagechange', function () {
    if (tokenUsageState.controller) return;
    const page = document.getElementById('page-settings');
    const section = document.getElementById('settings-section-token-usage');
    if (page && page.classList.contains('active') && section && section.classList.contains('active')) {
        renderTokenUsageAnalysis(tokenUsageState.result);
    }
});

function renderTokenUsageAnalysis(res) {
    const host = document.getElementById('settings-token-analysis');
    if (!host) return;
    host.replaceChildren();
    if (!res || !res.summary) {
        host.textContent = tokenUsageText('dashboard.usageUnavailable', null, '用量统计暂不可用');
        return;
    }
    const exact = value => tokenUsageCount(value).toLocaleString(window.__locale || 'zh-CN');
    const label = key => tokenUsageText('dashboard.' + key, null, key);
    const metrics = document.createElement('div');
    metrics.className = 'token-usage-metrics';
    tokenUsageMetrics(res.summary).forEach(([key, value]) => {
        const card = document.createElement('div');
        const title = document.createElement('span');
        title.textContent = label(key);
        const count = document.createElement('strong');
        count.textContent = typeof value === 'string' ? value : exact(value);
        card.append(title, count);
        metrics.append(card);
    });
    host.append(metrics);
    if (!tokenUsageCount(res.summary.events)) {
        const empty = document.createElement('p');
        empty.textContent = tokenUsageText('dashboard.noTokenUsageYet', null, '暂无用量');
        host.append(empty);
        return;
    }
    // Tables use text nodes: provider model names and conversation IDs are untrusted.
    const table = (titleKey, headings, rows) => {
        const section = document.createElement('section');
        const title = document.createElement('h4');
        title.textContent = label(titleKey);
        const wrap = document.createElement('div');
        wrap.className = 'token-usage-table-wrap';
        const grid = document.createElement('table');
        const caption = document.createElement('caption');
        caption.className = 'sr-only';
        caption.textContent = title.textContent;
        const head = document.createElement('thead');
        const hr = document.createElement('tr');
        headings.forEach(key => {
            const th = document.createElement('th');
            th.scope = 'col';
            th.textContent = label(key);
            hr.append(th);
        });
        head.append(hr);
        const body = document.createElement('tbody');
        rows.forEach(values => {
            const tr = document.createElement('tr');
            values.forEach(value => {
                const td = document.createElement('td');
                td.textContent = String(value == null ? '' : value);
                tr.append(td);
            });
            body.append(tr);
        });
        grid.append(caption, head, body);
        wrap.append(grid);
        section.append(title, wrap);
        host.append(section);
    };
    const counts = row => [exact(row.promptTokens), exact(row.completionTokens), exact(row.cachedTokens),
        exact(row.cacheWriteTokens), exact(row.reasoningTokens), exact(row.totalTokens), exact(row.modelCalls)];
    const fields = ['usageInput', 'usageOutput', 'usageCacheRead', 'usageCacheWrite', 'usageReasoning', 'usageTotal', 'usageCalls'];
    table('usageByModel', ['usageModel', 'usageShare', ...fields], (res.byModel || []).map(row => [row.label || row.key,
        tokenUsageCount(res.summary.totalTokens) > 0 ? (tokenUsageCount(row.totalTokens) / tokenUsageCount(res.summary.totalTokens) * 100).toFixed(1) + '%' : '—', ...counts(row)]));
    const trendTitle = document.createElement('h4');
    trendTitle.textContent = label('usageTrend');
    const trend = document.createElement('div');
    trend.className = 'token-usage-trend';
    const days = (res.byDay || []).slice().reverse();
    const peak = Math.max(1, ...days.map(row => tokenUsageCount(row.totalTokens)));
    days.forEach(row => {
        const bar = document.createElement('div');
        bar.className = 'token-usage-day';
        const count = document.createElement('span');
        count.textContent = formatTokenUsageCompact(row.totalTokens);
        const fill = document.createElement('div');
        fill.className = 'token-usage-day-bar';
        fill.style.height = Math.max(1, tokenUsageCount(row.totalTokens) / peak * 90) + 'px';
        const date = document.createElement('span');
        date.textContent = String(row.key || '').slice(5);
        bar.title = row.key + ': ' + exact(row.totalTokens);
        bar.append(count, fill, date);
        trend.append(bar);
    });
    host.append(trendTitle, trend);
    table('usageByDay', ['usageDate', ...fields], (res.byDay || []).map(row => [row.key, ...counts(row)]));
    table('usageRecent', ['usageTime', 'usageModel', 'usageStatus', 'usageTotal', 'usageCalls'], (res.recent || []).map(row => {
        const time = new Date(row.createdAt);
        return [Number.isFinite(time.getTime()) ? time.toLocaleString(window.__locale || 'zh-CN') : '—',
            row.model || '—', row.reason === 'final' ? label('usageFinal') : row.reason === 'partial' ? label('usagePartial') : (row.reason || '—'), exact(row.totalTokens), exact(row.modelCalls)];
    }));
}
