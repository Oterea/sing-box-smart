// Presentation-only state: callers supply ranked nodes; probing and scoring
// always keep the complete backend snapshot, independent of this window.
const SmartNodeList = (() => {
 function create(limit=30) {
  let airport, expanded=false;
  return {
   view(nodes,airportID) {
    if(airport!==airportID){airport=airportID;expanded=false;}
    const visible=expanded?nodes:nodes.slice(0,limit);
    return {nodes:visible,total:nodes.length,visible:visible.length,remaining:nodes.length-visible.length,expanded,canToggle:nodes.length>limit};
   },
   toggle(){expanded=!expanded;}
  };
 }
 return {create};
})();
