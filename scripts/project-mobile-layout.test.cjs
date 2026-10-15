/* Offline browser regression for the project header overlap and tab hit targets.
 * Requires Playwright and Chrome (or CHROME_PATH); no server/login/data is used.
 * Run: node --test scripts/project-mobile-layout.test.cjs
 */
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { test } = require('node:test');
const { chromium } = require('playwright');

const root = path.resolve(__dirname, '..');
const read = file => fs.readFileSync(path.join(root, file), 'utf8');
const template = read('web/templates/index.html');
const start = template.indexOf('<main class="projects-detail"');
assert.ok(start >= 0, 'project detail fixture must exist');
const markup = template.slice(start, template.indexOf('</main>', start) + 7);
const source = read('web/static/js/projects.js');
const switchTab = source.slice(source.indexOf('function switchProjectTab('), source.indexOf('async function loadProjectAssets('));
const tabs = ['facts', 'graph', 'assets', 'conversations', 'vulns', 'settings'];

test('project headers contain their content and all tabs receive mobile taps', async () => {
    const browser = await chromium.launch({
        headless: true,
        ...(process.env.CHROME_PATH ? { executablePath: process.env.CHROME_PATH } : {}),
    });
    try {
        for (const width of [320, 390, 430, 767, 768, 900, 1440]) {
            const context = await browser.newContext({ viewport: { width, height: 900 }, isMobile: width < 768, hasTouch: true });
            try {
                const page = await context.newPage();
                // Keep the real markup and both complete stylesheets, including theme overrides.
                await page.setContent(`<html><head><meta name="viewport" content="width=device-width, initial-scale=1"></head><body><div class="container"><header><div class="header-content">CyberStrikeAI</div></header><div id="page-projects" class="page active"><div class="page-content projects-page-layout">${markup}</div></div></div></body></html>`);
                for (const file of ['style.css', 'mobile.css']) await page.addStyleTag({ content: read(`web/static/css/${file}`) });
                // Use the real tab handler; replace only data loaders to keep the test offline.
                await page.addScriptTag({ content: `let currentProjectTab = 'facts'; window.loads = [];
                    ${['Facts', 'FactGraph', 'Assets', 'Conversations', 'Vulnerabilities'].map(name => `function loadProject${name}() { window.loads.push('${name}'); }`).join('\n')}
                    ${switchTab}` });
                for (const locale of ['zh-CN', 'en-US', 'ru-RU']) {
                    const translations = JSON.parse(read(`web/static/i18n/${locale}.json`));
                    await page.evaluate(dict => {
                        document.querySelectorAll('[data-i18n]').forEach(el => {
                            if (el.children.length || el.hasAttribute('data-i18n-attr')) return;
                            const value = el.dataset.i18n.split('.').reduce((obj, key) => obj?.[key], dict);
                            if (typeof value === 'string') el.textContent = value;
                        });
                        document.querySelector('.projects-detail').classList.add('has-project');
                        document.querySelector('#projects-detail-inner').hidden = false;
                        document.querySelector('#projects-detail-title').textContent = '玉米 — Long project name / '.repeat(8);
                        const desc = document.querySelector('#projects-detail-desc');
                        desc.hidden = false;
                        desc.textContent = 'Long project description / 项目描述 '.repeat(20);
                        document.querySelector('#project-stat-sparse').hidden = false;
                        document.querySelector('#projects-detail-meta-time').textContent = '2 days ago';
                    }, translations);
                    for (const theme of ['light', 'dark']) {
                        const label = `${width}px ${locale} ${theme}`;
                        await page.evaluate(theme => { document.documentElement.dataset.theme = theme; }, theme);
                        await page.locator('.projects-detail-header').scrollIntoViewIfNeeded();
                        const layout = await page.evaluate(() => {
                            const header = document.querySelector('.projects-detail-header');
                            const bounds = header.getBoundingClientRect();
                            const children = ['.projects-detail-headline', '.projects-detail-desc', '.projects-detail-header-actions'];
                            return {
                                contained: children.every(selector => {
                                    const r = document.querySelector(selector).getBoundingClientRect();
                                    return r.top >= bounds.top && r.bottom <= bounds.bottom;
                                }),
                                separate: document.querySelector('.projects-tabs').getBoundingClientRect().top >= bounds.bottom,
                                appHeight: document.querySelector('.container > header').getBoundingClientRect().height,
                                overflow: document.documentElement.scrollWidth > innerWidth,
                            };
                        });
                        assert.ok(layout.contained, `${label}: header content escapes its bounds`);
                        assert.ok(layout.separate, `${label}: header overlaps tabs`);
                        assert.equal(layout.overflow, false, `${label}: page overflows horizontally`);
                        if (width < 768) {
                            assert.equal(layout.appHeight, 52, `${label}: app bar height changed`);
                            assert.ok(await page.locator('#project-facts-search').evaluate(input => {
                                const bounds = input.getBoundingClientRect();
                                const icon = document.querySelector('.projects-fact-search-icon').getBoundingClientRect();
                                const textStart = bounds.left + parseFloat(getComputedStyle(input).paddingLeft);
                                return textStart > icon.right && Math.abs((icon.top + icon.bottom - bounds.top - bounds.bottom) / 2) < 1;
                            }), `${label}: search icon overlaps text or is off center`);
                        }
                        for (const tab of [...tabs, 'facts']) {
                            const button = page.locator(`#project-tab-${tab}`);
                            await button.scrollIntoViewIfNeeded();
                            await button.tap({ timeout: 3000 });
                            assert.ok(await button.evaluate(el => {
                                const r = el.getBoundingClientRect();
                                const nav = el.parentElement.getBoundingClientRect();
                                const x = (Math.max(r.left, nav.left, 0) + Math.min(r.right, nav.right, innerWidth)) / 2;
                                return el.contains(document.elementFromPoint(x, r.y + r.height / 2));
                            }), `${label}: ${tab} hit target is covered`);
                            await assert.doesNotReject(() => page.waitForFunction(tab => {
                                return document.querySelector(`#project-tab-${tab}`).classList.contains('is-active') &&
                                    [...document.querySelectorAll('.projects-panel')].every(panel => panel.hidden === (panel.id !== `project-panel-${tab}`));
                            }, tab, { timeout: 1000 }), `${label}: ${tab} failed to switch`);
                        }
                    }
                }
            } finally {
                await context.close();
            }
        }
    } finally {
        await browser.close();
    }
});
