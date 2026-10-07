document.addEventListener("securitypolicyviolation", (event) => {
  console.error("securitypolicyviolation: " + event.violatedDirective + " " + event.blockedURI);
});
