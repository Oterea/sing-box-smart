const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

const context = vm.createContext({});
vm.runInContext(fs.readFileSync('web/stability-detail.js', 'utf8'), context);

test('详情汇总指标和原始样本使用独立更新键', () => {
 const summary={samples:10,successes:10,failures:0,success_rate:1,p50_ms:100,p90_ms:150,high_latency_rate:0,max_consecutive_failures:0};
 const first=[{at:'2026-10-21T00:00:00Z',success:true,delay_ms:100}];
 const second=[...first,{at:'2026-10-21T00:00:03Z',success:true,delay_ms:110}];
 assert.equal(context.stabilitySummaryKey(summary),context.stabilitySummaryKey({...summary}));
 assert.notEqual(context.stabilitySamplesKey(first),context.stabilitySamplesKey(second));
});

test('样本键包含失败和延迟变化', () => {
 const first=[{at:'2026-10-21T00:00:00Z',success:true,delay_ms:100}];
 const failed=[{at:'2026-10-21T00:00:00Z',success:false,error:'timeout'}];
 assert.notEqual(context.stabilitySamplesKey(first),context.stabilitySamplesKey(failed));
});

test('样本键只依赖窗口边界，新增样本能触发更新', () => {
 const middle=Array.from({length:10000},(_,i)=>({at:`2026-10-21T00:${String(Math.floor(i/60)).padStart(2,'0')}:${String(i%60).padStart(2,'0')}Z`,success:true,delay_ms:100}));
 const next=[...middle,{at:'2026-10-21T03:00:00Z',success:true,delay_ms:110}];
 assert.notEqual(context.stabilitySamplesKey(middle),context.stabilitySamplesKey(next));
});
