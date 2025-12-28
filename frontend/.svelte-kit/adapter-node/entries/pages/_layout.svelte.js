import "clsx";
import { a as api } from "../../chunks/api.js";
function _layout($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let { children } = $$props;
    let isAuthenticated = api.isAuthenticated;
    $$renderer2.push(`<div class="app svelte-12qhfyh">`);
    if (isAuthenticated) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<nav class="navbar svelte-12qhfyh"><div class="container navbar-content svelte-12qhfyh"><a href="/" class="logo svelte-12qhfyh">Open Accounting</a> <div class="nav-links svelte-12qhfyh"><a href="/dashboard" class="svelte-12qhfyh">Dashboard</a> <a href="/accounts" class="svelte-12qhfyh">Accounts</a> <a href="/journal" class="svelte-12qhfyh">Journal</a> <a href="/contacts" class="svelte-12qhfyh">Contacts</a> <a href="/invoices" class="svelte-12qhfyh">Invoices</a> <a href="/payments" class="svelte-12qhfyh">Payments</a> <a href="/reports" class="svelte-12qhfyh">Reports</a> <button class="btn btn-secondary">Logout</button></div></div></nav>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <main class="main-content svelte-12qhfyh">`);
    children($$renderer2);
    $$renderer2.push(`<!----></main></div>`);
  });
}
export {
  _layout as default
};
