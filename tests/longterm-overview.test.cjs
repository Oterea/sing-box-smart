const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');

const context=vm.createContext({document:{getElementById:()=>null}});
vm.runInContext(fs.readFileSync('web/stability-view.js','utf8'),context);
vm.runInContext(fs.readFileSync('web/longterm-overview.js','utf8'),context);

function node(id,summary){return {id,name:id,windows:[{days:14,summary}]};}

test('长期表现模块可在 app.js 之前独立初始化',()=>{
 assert.equal(vm.runInContext('typeof overviewElement',context),'function');
 assert.equal(vm.runInContext('typeof renderLongTermOverview',context),'function');
});

test('长期表现按指标方向计算独立排名并将样本不足排除',()=>{
 const nodes=[
  node('a',{samples:20,p50_ms:100,p90_ms:200,success_rate:.99,high_latency_rate:.01}),
  node('b',{samples:20,p50_ms:150,p90_ms:180,success_rate:1,high_latency_rate:0}),
  node('c',{samples:9,p50_ms:50,p90_ms:60,success_rate:1,high_latency_rate:0})
 ];
 const ranks=context.overviewRanks(nodes,14);
 assert.equal(ranks.get('a').p50,1);
 assert.equal(ranks.get('b').p50,2);
 assert.equal(ranks.get('b').success,1);
 assert.equal(ranks.get('a').success,2);
 assert.equal(ranks.has('c'),false);
});

test('组内多指标排序使用固定优先级且窗口汇总可切换',()=>{
 const nodes=[node('a',{samples:20,p50_ms:100,p90_ms:300,success_rate:.99,high_latency_rate:.01}),node('b',{samples:20,p50_ms:100,p90_ms:200,success_rate:1,high_latency_rate:0})];
 vm.runInContext("longTermOverviewSorts=['p50','p90']",context);
 const sorted=context.overviewSort(nodes,14).map(item=>item.id);
 assert.deepEqual([...sorted],['b','a']);
 assert.equal(context.overviewSummary(nodes[0],14).p50_ms,100);
});
