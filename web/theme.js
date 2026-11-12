(function(){
 const key='sing-box-smart-theme';
 const valid=new Set(['light','dark','system']);
 let choice=localStorage.getItem(key)||'system';
 if(!valid.has(choice))choice='system';
 function apply(value){
  choice=value;
  if(value==='system')document.documentElement.removeAttribute('data-theme');
  else document.documentElement.dataset.theme=value;
  localStorage.setItem(key,value);
  for(const button of document.querySelectorAll('[data-theme-choice]')){
   const active=button.dataset.themeChoice===value;
   button.classList.toggle('active',active);
   button.setAttribute('aria-pressed',String(active));
  }
 }
 apply(choice);
 window.addEventListener('DOMContentLoaded',function(){
  apply(choice);
  document.getElementById('theme-switch')?.addEventListener('click',function(event){
   const button=event.target.closest('[data-theme-choice]');
   if(button)apply(button.dataset.themeChoice);
  });
 });
})();
