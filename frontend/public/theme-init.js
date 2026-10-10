// Applies the saved theme before first paint to avoid a light/dark flash.
// Kept as an external file so the Content-Security-Policy can forbid inline scripts.
try {
  var t = localStorage.getItem('theme') || 'system'
  var dark = t === 'dark' || (t === 'system' && matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', dark)
} catch (e) {
  // storage unavailable: keep the default (light) theme
}
