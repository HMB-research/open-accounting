import { W as store_get, U as head, V as attr, X as unsubscribe_stores } from "../../../chunks/index2.js";
import { p as page } from "../../../chunks/stores.js";
import "decimal.js";
import { e as escape_html } from "../../../chunks/context.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let tenantId = store_get($$store_subs ??= {}, "$page", page).url.searchParams.get("tenant") || "";
    let isLoading = false;
    let asOfDate = (/* @__PURE__ */ new Date()).toISOString().split("T")[0];
    let selectedReport = "trial-balance";
    head("2pp8mk", $$renderer2, ($$renderer3) => {
      $$renderer3.title(($$renderer4) => {
        $$renderer4.push(`<title>Reports - Open Accounting</title>`);
      });
    });
    $$renderer2.push(`<div class="container"><div class="header svelte-2pp8mk"><h1 class="svelte-2pp8mk">Financial Reports</h1> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--></div> `);
    if (!tenantId) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="card empty-state svelte-2pp8mk"><p>Please select a tenant from the <a href="/dashboard">dashboard</a>.</p></div>`);
    } else {
      $$renderer2.push("<!--[!-->");
      $$renderer2.push(`<div class="report-controls card svelte-2pp8mk"><div class="control-row svelte-2pp8mk"><div class="form-group svelte-2pp8mk"><label class="label" for="reportType">Report Type</label> `);
      $$renderer2.select({ class: "input", id: "reportType", value: selectedReport }, ($$renderer3) => {
        $$renderer3.option({ value: "trial-balance" }, ($$renderer4) => {
          $$renderer4.push(`Trial Balance`);
        });
        $$renderer3.option({ value: "balance-sheet", disabled: true }, ($$renderer4) => {
          $$renderer4.push(`Balance Sheet (coming soon)`);
        });
        $$renderer3.option({ value: "income-statement", disabled: true }, ($$renderer4) => {
          $$renderer4.push(`Income Statement (coming soon)`);
        });
      });
      $$renderer2.push(`</div> <div class="form-group svelte-2pp8mk"><label class="label" for="asOfDate">As of Date</label> <input class="input" type="date" id="asOfDate"${attr("value", asOfDate)}/></div> <button class="btn btn-primary"${attr("disabled", isLoading, true)}>${escape_html("Generate Report")}</button></div></div> `);
      {
        $$renderer2.push("<!--[!-->");
      }
      $$renderer2.push(`<!--]--> `);
      {
        $$renderer2.push("<!--[!-->");
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--></div>`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}
export {
  _page as default
};
