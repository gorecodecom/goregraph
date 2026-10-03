(function(){
  'use strict';
  const storageKey='goregraph.dashboard.theme';
  const choices=['light','dark','system'];
  const media=window.matchMedia?.('(prefers-color-scheme: dark)');
  let preference='light';
  try {
    const saved=window.localStorage.getItem(storageKey);
    if(choices.includes(saved))preference=saved;
  } catch {}
  function applyTheme() {
    document.documentElement.dataset.theme=preference==='system'?(media?.matches?'dark':'light'):preference;
    document.documentElement.dataset.themePreference=preference;
    document.querySelectorAll('#dashboard-theme [data-theme-choice]').forEach(button=>{
      button.setAttribute('aria-pressed',String(button.dataset.themeChoice===preference));
    });
  }
  applyTheme();
  media?.addEventListener('change',()=>{if(preference==='system')applyTheme();});
  function initializeControl() {
    const control=document.getElementById('dashboard-theme');
    if(!control)return;
    applyTheme();
    control.addEventListener('click',event=>{
      const button=event.target.closest('button[data-theme-choice]');
      if(!button||!control.contains(button)||!choices.includes(button.dataset.themeChoice))return;
      preference=button.dataset.themeChoice;
      applyTheme();
      try { window.localStorage.setItem(storageKey,preference); } catch {}
    });
    window.addEventListener('storage',event=>{
      if(event.key!==storageKey)return;
      preference=choices.includes(event.newValue)?event.newValue:'light';
      applyTheme();
    });
  }
  if(document.readyState==='loading')document.addEventListener('DOMContentLoaded',initializeControl,{once:true});
  else initializeControl();
})();
