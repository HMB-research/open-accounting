import { U as head, V as attr, W as store_get, X as ensure_array_like, Y as attr_class, Z as stringify, _ as unsubscribe_stores } from './index2-VjzSvL4X.js';
import { p as page } from './stores-DMzxCuNa.js';
import { a as api } from './api-BA0AUtWR.js';
import { e as escape_html } from './context-Cv9QAF3V.js';
import './false-CRHihH2U.js';
import './exports-CgQJUv15.js';
import './state.svelte-6rJr4dnJ.js';
import 'decimal.js';

function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let contacts = [];
    let isLoading = true;
    let error = "";
    let filterType = "";
    let searchQuery = "";
    async function loadContacts(tenantId) {
      isLoading = true;
      error = "";
      try {
        contacts = await api.listContacts(tenantId, {
          type: filterType || void 0,
          search: searchQuery || void 0,
          active_only: true
        });
      } catch (err) {
        error = err instanceof Error ? err.message : "Failed to load contacts";
      } finally {
        isLoading = false;
      }
    }
    async function handleSearch() {
      const tenantId = store_get($$store_subs ??= {}, "$page", page).url.searchParams.get("tenant");
      if (tenantId) {
        loadContacts(tenantId);
      }
    }
    const typeLabels = { CUSTOMER: "Customer", SUPPLIER: "Supplier", BOTH: "Both" };
    const typeBadgeClass = {
      CUSTOMER: "badge-customer",
      SUPPLIER: "badge-supplier",
      BOTH: "badge-both"
    };
    head("67057e", $$renderer2, ($$renderer3) => {
      $$renderer3.title(($$renderer4) => {
        $$renderer4.push(`<title>Contacts - Open Accounting</title>`);
      });
    });
    $$renderer2.push(`<div class="container"><div class="header svelte-67057e"><h1 class="svelte-67057e">Contacts</h1> <button class="btn btn-primary">+ New Contact</button></div> <div class="filters card svelte-67057e"><div class="filter-row svelte-67057e">`);
    $$renderer2.select({ class: "input", value: filterType, onchange: handleSearch }, ($$renderer3) => {
      $$renderer3.option({ value: "" }, ($$renderer4) => {
        $$renderer4.push(`All Types`);
      });
      $$renderer3.option({ value: "CUSTOMER" }, ($$renderer4) => {
        $$renderer4.push(`Customers`);
      });
      $$renderer3.option({ value: "SUPPLIER" }, ($$renderer4) => {
        $$renderer4.push(`Suppliers`);
      });
      $$renderer3.option({ value: "BOTH" }, ($$renderer4) => {
        $$renderer4.push(`Both`);
      });
    });
    $$renderer2.push(` <input class="input search-input svelte-67057e" type="text" placeholder="Search contacts..."${attr("value", searchQuery)}/> <button class="btn btn-secondary">Search</button></div></div> `);
    if (error) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="alert alert-error">${escape_html(error)}</div>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    if (isLoading) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<p>Loading contacts...</p>`);
    } else {
      $$renderer2.push("<!--[!-->");
      if (contacts.length === 0) {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<div class="empty-state card svelte-67057e"><p>No contacts found. Create your first contact to get started.</p></div>`);
      } else {
        $$renderer2.push("<!--[!-->");
        $$renderer2.push(`<div class="card"><table class="table"><thead><tr><th>Name</th><th>Type</th><th>Email</th><th>Phone</th><th>VAT Number</th><th>Payment Terms</th></tr></thead><tbody><!--[-->`);
        const each_array = ensure_array_like(contacts);
        for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
          let contact = each_array[$$index];
          $$renderer2.push(`<tr${attr_class("svelte-67057e", void 0, { "inactive": !contact.is_active })}><td class="name svelte-67057e">${escape_html(contact.name)}</td><td><span${attr_class(`badge ${stringify(typeBadgeClass[contact.contact_type])}`, "svelte-67057e")}>${escape_html(typeLabels[contact.contact_type])}</span></td><td class="email svelte-67057e">${escape_html(contact.email || "-")}</td><td>${escape_html(contact.phone || "-")}</td><td class="vat svelte-67057e">${escape_html(contact.vat_number || "-")}</td><td>${escape_html(contact.payment_terms_days)} days</td></tr>`);
        }
        $$renderer2.push(`<!--]--></tbody></table></div>`);
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--></div> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]-->`);
    if ($$store_subs) unsubscribe_stores($$store_subs);
  });
}

export { _page as default };
//# sourceMappingURL=_page.svelte-By1IXbsE.js.map
