(function() {
  let theme;
  try {
    theme = localStorage.getItem('loom_theme');
  } catch (e) {}
  if (theme === 'dark' || (!theme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    document.documentElement.classList.add('dark');
  } else {
    document.documentElement.classList.remove('dark');
  }
})();
