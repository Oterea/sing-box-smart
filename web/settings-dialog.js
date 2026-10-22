// Dialog visibility is separate from form editing: closing preserves drafts.
(() => {
 const dialog=document.getElementById('settings-dialog');
 const open=document.getElementById('settings-open');
 const close=document.getElementById('settings-close');
 const toast=document.getElementById('toast');
 if(!dialog||!open||!close)return;
 open.addEventListener('click',()=>{
  if(dialog.open)return;
  if(toast)dialog.append(toast);
  dialog.showModal();
  dialog.focus({preventScroll:true});
  document.body.classList.add('settings-open');
 });
 close.addEventListener('click',()=>dialog.close());
 dialog.addEventListener('close',()=>{
  document.body.classList.remove('settings-open');
  if(toast)document.body.append(toast);
  open.focus();
 });
 // Only a click that starts and ends on the backdrop dismisses the dialog.
 const outside=event=>{
  const box=dialog.getBoundingClientRect();
  return event.target===dialog&&(event.clientX<box.left||event.clientX>box.right||event.clientY<box.top||event.clientY>box.bottom);
 };
 let backdrop=false;
 dialog.addEventListener('pointerdown',event=>{backdrop=outside(event);});
 dialog.addEventListener('click',event=>{if(backdrop&&outside(event))dialog.close();backdrop=false;});
})();
