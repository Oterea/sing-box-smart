const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const source=fs.readFileSync('web/app.js','utf8');
const format=vm.runInNewContext(source.slice(source.indexOf('const time ='),source.indexOf('const tone ='))+'\n({recordTime,elapsedTime})');

test('sleep durations switch to hours at 60 minutes and retain remaining minutes and seconds',()=>{
 for(const [seconds,expected] of [[0,'0 秒'],[59,'59 秒'],[60,'1 分钟'],[3599,'59 分钟 59 秒'],[3600,'1 小时'],[3601,'1 小时 1 秒'],[3660,'1 小时 1 分钟'],[45905,'12 小时 45 分钟 5 秒'],[90000,'25 小时'],[-1,'0 秒'],[Infinity,'0 秒']])assert.equal(format.elapsedTime(seconds),expected);
});
test('record timestamps include the local calendar date for cross-day and cross-year records',()=>{
 assert.match(format.recordTime(new Date(2026,9,18,20,4,2)),/^2026-10-18 20:04:02$/);
 assert.match(format.recordTime(new Date(2026,9,19,8,49,7)),/^2026-10-19 08:49:07$/);
 assert.match(format.recordTime(new Date(2027,0,1,1,2,3)),/^2027-01-01 01:02:03$/);
 assert.equal(format.recordTime('invalid'),'—');
});
