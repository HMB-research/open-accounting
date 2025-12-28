import { U as head } from "../../../chunks/index2.js";
import "@sveltejs/kit/internal";
import "../../../chunks/exports.js";
import "../../../chunks/utils.js";
import "clsx";
import "@sveltejs/kit/internal/server";
import "../../../chunks/state.svelte.js";
import "decimal.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    head("11yx1mx", $$renderer2, ($$renderer3) => {
      $$renderer3.title(($$renderer4) => {
        $$renderer4.push(`<title>Chart of Accounts - Open Accounting</title>`);
      });
    });
    $$renderer2.push(`<div class="container"><div class="header svelte-11yx1mx"><h1 class="svelte-11yx1mx">Chart of Accounts</h1> <button class="btn btn-primary">+ New Account</button></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<p>Loading accounts...</p>`);
    }
    $$renderer2.push(`<!--]--></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]-->`);
  });
}
export {
  _page as default
};
