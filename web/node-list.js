// Presentation-only state: callers supply ranked nodes; probing and scoring
// always keep the complete backend snapshot, independent of this window.
const SmartNodeList = (() => {
 function create(limit=30) {
  let airport, expanded=false;
  return {
   view(nodes,airportID,configuredLimit=limit) {
    const count=Number.isInteger(configuredLimit)&&configuredLimit>=1&&configuredLimit<=1000?configuredLimit:limit;
    if(airport!==airportID){airport=airportID;expanded=false;}
    const visible=expanded?nodes:nodes.slice(0,count);
    return {limit:count,nodes:visible,total:nodes.length,visible:visible.length,remaining:nodes.length-visible.length,expanded,canToggle:nodes.length>count};
   },
   toggle(){expanded=!expanded;}
  };
 }
 return {create};
})();
