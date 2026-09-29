const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

const source = fs.readFileSync('web/app.js', 'utf8');
const context = vm.createContext({document:{}});
vm.runInContext(source.slice(0, source.indexOf('function render(')), context);

function bodyWithRows(nodes, best, state) {
 const body = {
  children: [], moves: 0, scans: 0,
  querySelectorAll(selector) {
   this.scans++;
   assert.equal(selector, 'tr[data-row-id]');
   return this.children;
  },
  insertBefore(row, next) {
   this.moves++;
   const old = this.children.indexOf(row);
   if (old !== -1) this.children.splice(old, 1);
   const index = next ? this.children.indexOf(next) : -1;
   this.children.splice(index === -1 ? this.children.length : index, 0, row);
  },
  get firstElementChild() { return this.children[0] ?? null; },
 };
 for (const node of nodes) {
  const row = {
   dataset:{rowId:node.id,renderKey:context.nodeKey(node,best,state)},
   get nextElementSibling() { return body.children[body.children.indexOf(this)+1] ?? null; },
   remove() { body.children.splice(body.children.indexOf(this), 1); },
  };
  body.children.push(row);
 }
 return body;
}

test('unchanged rows are not moved or rescanned individually', () => {
 const nodes=Array.from({length:80},(_,i)=>({id:`n${i}`,name:`Node ${i}`,checks:2,score:100+i,delay_ms:100+i,last_success:true,history:[]}));
 const state={current_id:'n0',phase:'normal',api_healthy:true,pending_id:''};
 const body=bodyWithRows(nodes,100,state);
 context.syncRows(body,nodes,100,state);
 assert.equal(body.moves,0);
 assert.equal(body.scans,1);
 assert.deepEqual(body.children.map(row=>row.dataset.rowId),nodes.map(node=>node.id));
});

test('only a changed ordering moves rows', () => {
 const nodes=[{id:'a',score:100,history:[]},{id:'b',score:110,history:[]},{id:'c',score:120,history:[]}];
 const state={current_id:'a',phase:'normal',api_healthy:true,pending_id:''};
 const body=bodyWithRows(nodes,100,state);
 context.syncRows(body,[nodes[1],nodes[0],nodes[2]],100,state);
 assert.equal(body.moves,1);
 assert.deepEqual(body.children.map(row=>row.dataset.rowId),['b','a','c']);
});

test('small best-score changes without color changes keep row keys stable', () => {
 const node={id:'a',score:100,history:[{score:100},{score:120}]};
 const state={current_id:'a',phase:'normal',api_healthy:true,pending_id:''};
 assert.equal(context.nodeKey(node,100,state),context.nodeKey(node,101,state));
 assert.notEqual(context.nodeKey(node,100,state),context.nodeKey(node,70,state));
});

test('mobile top card ends with a single-column override', () => {
 const css=fs.readFileSync('web/styles.css','utf8');
 const desktop=css.lastIndexOf('.top-card{grid-template-columns:minmax(0,1fr) minmax(0,1fr)');
 const mobile=css.lastIndexOf('@media(max-width:700px){.top-card{grid-template-columns:minmax(0,1fr)');
 assert.ok(desktop>=0 && mobile>desktop);
});

test('header stays informational and the manual check sits in the health card', () => {
 const html=fs.readFileSync('web/index.html','utf8');
 const header=html.slice(html.indexOf('<header>'),html.indexOf('</header>'));
 assert.doesNotMatch(header,/brand-mark|recheck/);
 assert.match(html.slice(html.indexOf('<section class="top-card'),html.indexOf('</section>')),/id="recheck"/);
});

test('active controls use the cyan palette and balanced health text', () => {
 const css=fs.readFileSync('web/styles.css','utf8');
 assert.match(css,/--blue:#42D3F2/);
 assert.match(css,/\.node-table \.node-choice\{[^}]*text-align:center/);
 assert.match(css,/\.top-health #recheck\{[^}]*background:var\(--blue\)[^}]*box-shadow:0 0 6px/);
 assert.match(css,/\.health-number span\{font-size:16px/);
});

test('table cells and policy-group heading share the intended alignment and scale', () => {
 const css=fs.readFileSync('web/styles.css','utf8');
 assert.match(css,/\.node-table th,\.node-table td\{text-align:center\}/);
 assert.match(css,/\.top-airports \.card-label\{font-size:17px;font-weight:650/);
});
