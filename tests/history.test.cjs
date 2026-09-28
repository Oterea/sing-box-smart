const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync('web/app.js', 'utf8');
const context = vm.createContext({});
vm.runInContext(source.slice(0, source.indexOf('function render(')), context);
const history = context.history;
const sample = (value, success = true) => ({ at: '2026-09-28T00:00:00Z', score: value, delay_ms: success ? value : null, success, error: 'timeout' });
const heights = html => [...html.matchAll(/class="chart-bar[^\"]*"[^>]* height="([^\"]+)"/g)].map(m => Number(m[1]));

test('柱高与实际值成比例，并从同一基线向上绘制', () => {
 const html = history([100, 200, 400].map(v => sample(v)), 'delay', 100);
 assert.deepEqual(heights(html), [11.75, 23.5, 47]);
 for (const m of html.matchAll(/class="chart-bar[^\"]*"[^>]* y="([^\"]+)"[^>]* height="([^\"]+)"/g)) {
  assert.equal(Number(m[1]) + Number(m[2]), 50);
 }
 assert.match(html, /bar-good/);
 assert.match(html, /bar-medium/);
});

test('严格 CSP 下不依赖行内样式或脚本，直接输出 SVG 几何属性', () => {
 const html = history([sample(200), sample(600)], 'score', 200);
 assert.doesNotMatch(html, /style=|<script|onload=/i);
 assert.deepEqual(heights(html), [47 / 3, 47]);
 const server = fs.readFileSync('internal/httpapi/server.go', 'utf8');
 assert.match(server, /style-src 'self'; script-src 'self'/);
});

test('失败没有伪造延迟；分数仍按实际值绘制', () => {
 assert.match(history([sample(200, false)], 'delay', 100), /chart-failure/);
 assert.deepEqual(heights(history([sample(200, false)], 'delay', 100)), []);
 assert.deepEqual(heights(history([sample(200, false), sample(400)], 'score', 100)), [23.5, 47]);
});

test('无记录和无分数正确显示，最多保留20条，没有占位假数据', () => {
 assert.match(history(null, 'score', 100), /暂无记录/);
 assert.match(history([sample(null, false)], 'score', 100), /chart-missing/);
 const html = history(Array.from({ length: 25 }, (_, i) => sample((i + 1) * 20)), 'delay', 100);
 assert.equal(heights(html).length, 20);
 assert.equal(heights(html)[0], 120 / 500 * 47);
 assert.doesNotMatch(html, /NaN|Infinity/);
});

test('当前两个数值共享相同字号，移除旧柱子的层叠样式', () => {
 const css = fs.readFileSync('web/styles.css', 'utf8');
 assert.match(css, /\.score-value,\.delay-value\{[^}]*font-size:21px/);
 assert.doesNotMatch(css, /\.sample\b/);
});
