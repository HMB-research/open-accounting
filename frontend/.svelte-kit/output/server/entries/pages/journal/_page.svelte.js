import { W as store_get, U as head, X as unsubscribe_stores } from "../../../chunks/index2.js";
import { p as page } from "../../../chunks/stores.js";
import "decimal.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let tenantId = store_get($$store_subs ??= {}, "$page", page).url.searchParams.get("tenant") || "";
    (/* @__PURE__ */ new Date()).toISOString().split("T")[0];
    head("1gt2yrk", $$renderer2, ($$renderer3) => {
      $$renderer3.title(($$renderer4) => {
        $$renderer4.push(`<title>Journal Entries - Open Accounting</title>`);
      });
    });
    $$renderer2.push(`<div class="container"><div class="header svelte-1gt2yrk"><h1 class="svelte-1gt2yrk">Journal Entries</h1> `);
    if (tenantId) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<button class="btn btn-primary">+ New Entry</button>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<p>Loading...</p>`);
    }
    $$renderer2.push(`<!--]--></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]-->`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
