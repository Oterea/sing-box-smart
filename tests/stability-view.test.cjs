const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

const context = vm.createContext({});
vm.runInContext(fs.readFileSync('web/stability-view.js', 'utf8'), context);

test('24 小时和长期视图共享相同的稳定性结论', () => {
 const thresholds=[200,600];
 const stable={samples:20,success_rate:1,p50_ms:150,p90_ms:250,high_latency_rate:0};
 const unstable={...stable,p90_ms:400};
 assert.deepEqual([...context.stabilityConclusion(stable,...thresholds)],['稳定低延迟','good']);
 assert.deepEqual([...context.stabilityConclusion(unstable,...thresholds)],['不稳定低延迟','bad']);
 assert.deepEqual([...context.stabilityConclusion({...stable,samples:9},...thresholds)],['样本不足','none']);
});
