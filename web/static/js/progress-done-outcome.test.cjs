const fs=require('node:fs'),vm=require('node:vm'),test=require('node:test'),assert=require('node:assert/strict');
const source=fs.readFileSync('web/static/js/monitor.js','utf8');const c=vm.createContext({});vm.runInContext(source.slice(source.indexOf('function progressDoneOutcome('),source.indexOf('function finalizeOutstandingToolCallsForProgress(')),c);
test('done preserves cancelled and failed outcomes instead of showing success',()=>{
 for(const status of ['cancelled','canceled','failed','timeout','cleanup_failed','cleanup_unconfirmed']){
  const result=c.progressDoneOutcome({status});assert.notEqual(result.icon,'✅');assert.equal(result.toolStatus,status.startsWith('cancel')?'cancelled':'failed');
 }
 assert.equal(c.progressDoneOutcome({status:'completed'}).icon,'✅');assert.equal(c.progressDoneOutcome({workflowStatus:'cancelled'}).icon,'⛔');
});
