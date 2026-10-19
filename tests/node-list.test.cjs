const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const {create}=vm.runInNewContext(fs.readFileSync('web/node-list.js','utf8')+'\nSmartNodeList');
const nodes=Array.from({length:70},(_,index)=>({id:String(index),score:index}));

test('collapsed list limits rendering without truncating or reordering the supplied snapshot',()=>{
 const list=create();
 for(const size of [0,1,29,30,31,70]){
  const input=nodes.slice(0,size),before=JSON.stringify(input),view=list.view(input,'a');
  assert.equal(view.visible,Math.min(size,30));assert.equal(view.total,size);assert.equal(view.remaining,Math.max(0,size-30));assert.equal(view.canToggle,size>30);
  assert.equal(JSON.stringify(view.nodes),JSON.stringify(input.slice(0,30)));assert.equal(JSON.stringify(input),before);
 }
});
test('expand choice survives live score and membership updates; collapse restores top thirty',()=>{
 const list=create();list.view(nodes,'a');list.toggle();
 assert.equal(list.view(nodes,'a').visible,70);
 const reranked=[...nodes].reverse();assert.equal(list.view(reranked,'a').nodes[0].id,'69');
 assert.equal(list.view(reranked.slice(0,20),'a').canToggle,false);
 assert.equal(list.view(reranked,'a').visible,70);
 list.toggle();assert.equal(list.view(reranked,'a').visible,30);assert.equal(list.view(reranked,'a').nodes[0].id,'69');
});
test('switching airports resets expansion, including returning to the previous airport',()=>{
 const list=create();list.view(nodes,'a');list.toggle();assert.equal(list.view(nodes,'a').expanded,true);
 assert.equal(list.view(nodes,'b').visible,30);list.toggle();assert.equal(list.view(nodes,'b').visible,70);
 assert.equal(list.view(nodes,'a').expanded,false);
});
