const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync('web/app.js', 'utf8');
const context = vm.createContext({});
vm.runInContext(source.slice(0, source.indexOf('function render(')), context);
const history = context.history;
const sample = (value, success = true) => ({ at: '2026-09-28T00:00:00Z', score: value, delay_ms: success ? value : null, success, error: 'timeout' });
const heights = html => [...html.matchAll(/class="chart-(?:bar|failure)[^\"]*"[^>]* height="([^\"]+)"/g)].map(m => Number(m[1]));

test('柱高与实际值成比例，并从同一基线向上绘制', () => {
 const html = history([100, 200, 400].map(v => sample(v)), 'delay', 100);
 assert.deepEqual(heights(html), [7.75, 15.5, 31]);
 for (const m of html.matchAll(/class="chart-bar[^\"]*"[^>]* y="([^\"]+)"[^>]* height="([^\"]+)"/g)) {
  assert.equal(Number(m[1]) + Number(m[2]), 33);
 }
 assert.match(html, /bar-good/);
 assert.match(html, /bar-medium/);
});

test('严格 CSP 下不依赖行内样式或脚本，直接输出 SVG 几何属性', () => {
 const html = history([sample(200), sample(600)], 'score', 200);
 assert.doesNotMatch(html, /style=|<script|onload=/i);
 assert.ok(Math.abs(heights(html)[0] / heights(html)[1] - 1 / 3) < 1e-10);
 const server = fs.readFileSync('internal/httpapi/server.go', 'utf8');
 assert.match(server, /style-src 'self'; script-src 'self'/);
});

test('失败没有伪造延迟；分数仍按实际值绘制', () => {
 assert.match(history([sample(200, false)], 'delay', 100), /chart-failure/);
 assert.deepEqual(heights(history([sample(200, false)], 'delay', 100)), [31]);
 const failedScore=history([sample(200, false), sample(400)], 'score', 100);
 assert.deepEqual(heights(failedScore), [15.5, 31]);
 assert.match(failedScore, /class="chart-failure"/);
 assert.doesNotMatch(failedScore, /chart-grid|chart-baseline|stroke-dasharray/);
});

test('无记录和无分数正确显示，最多保留20条，没有占位假数据', () => {
 assert.match(history(null, 'score', 100), /暂无记录/);
 assert.match(history([sample(null, false)], 'score', 100), /chart-failure/);
 const html = history(Array.from({ length: 25 }, (_, i) => sample((i + 1) * 20)), 'delay', 100);
 assert.equal(heights(html).length, 20);
 assert.equal(heights(html)[0], 120 / 500 * 31);
 assert.doesNotMatch(html, /NaN|Infinity/);
});

test('当前两个数值共享相同字号，移除旧柱子的层叠样式', () => {
 const css = fs.readFileSync('web/styles.css', 'utf8');
 assert.match(css, /\.node-table \.metric-cell\{[^}]*font-size:16px/);
 assert.doesNotMatch(css, /\.sample\b/);
});

test('选中区域与列表生成完全相同的表头与六个单元格', () => {
 const elements = new Map();
 const getElementById = id => {
  if (!elements.has(id)) elements.set(id, { textContent:'', innerHTML:'', parentElement:{dataset:{}}, classList:{toggle(){}} });
  return elements.get(id);
 };
 const renderContext = vm.createContext({document:{getElementById}});
 vm.runInContext(source.slice(0, source.indexOf('function toast(')), renderContext);
 const node = {id:'a',name:'Hong Kong 22',checks:20,score:300,delay_ms:200,last_success:true,current:true,history:[sample(200)]};
 const snapshot = {mode:'real',airports:[{id:'airport',name:'ToLink',selector:'tolink PIN'}],airport_id:'airport',current_id:'a',nodes:[node],api_healthy:true,phase:'normal',now:'2026-09-28T00:01:00Z',started_at:'2026-09-28T00:00:00Z',events:[]};
 renderContext.render(snapshot);
 const row = getElementById('selected-node').innerHTML;
 assert.equal(row, getElementById('nodes').innerHTML);
 assert.equal((row.match(/<td>/g)||[]).length, 6);
 assert.equal(getElementById('status-line').textContent, '');
 const header = renderContext.tableHeading();
 assert.equal((header.match(/scope="col"/g)||[]).length,6);
 assert.doesNotMatch(header, /colspan|rowspan|当前表现|历史走势|最近20/);
 assert.equal((fs.readFileSync('web/index.html','utf8').match(/class="node-table"/g)||[]).length,2);
});

test('选择按钮只在允许选择的状态启用，当前节点与启动状态禁用', () => {
 const n = {id:'n',name:'节点',checks:1,score:120,delay_ms:100,last_success:true,history:[]};
 const s = {current_id:'other',api_healthy:true,phase:'normal'};
 const button = state => context.nodeRow(n,120,state).match(/<button[^>]*>/)[0];
 assert.doesNotMatch(button(s), /disabled/);
 assert.match(button({...s,current_id:'n'}), /disabled/);
 assert.match(button({...s,phase:'startup'}), /disabled/);
 assert.match(button({...s,pending_id:'p'}), /disabled/);
 assert.match(button({...s,api_healthy:false}), /disabled/);
});
