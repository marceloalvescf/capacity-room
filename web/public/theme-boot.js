(function () {
  var root = document.documentElement;
  var dark = window.matchMedia("(prefers-color-scheme: dark)");
  var choice = "system";
  try {
    var stored = window.localStorage.getItem("kcr.theme");
    if (stored === "light" || stored === "dark") {
      choice = stored;
    }
  } catch {
    choice = "system";
  }

  function apply() {
    var current = root.getAttribute("data-theme-choice");
    root.setAttribute("data-theme", current === "system" ? (dark.matches ? "dark" : "light") : current);
  }

  root.setAttribute("data-theme-choice", choice);
  apply();
  dark.addEventListener("change", apply);
})();
