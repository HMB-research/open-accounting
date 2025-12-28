import { U as head, W as store_get, X as unsubscribe_stores, Y as ensure_array_like, Z as attr_class, _ as stringify } from "../../../chunks/index2.js";
import { p as page } from "../../../chunks/stores.js";
import { a as api } from "../../../chunks/api.js";
import "decimal.js";
import { e as escape_html } from "../../../chunks/context.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let invoices = [];
    let contacts = [];
    let isLoading = true;
    let error = "";
    let filterType = "";
    let filterStatus = "";
    (/* @__PURE__ */ new Date()).toISOString().split("T")[0];
    new Date(Date.now() + 14 * 24 * 60 * 60 * 1e3).toISOString().split("T")[0];
    async function loadData(tenantId) {
      isLoading = true;
      error = "";
      try {
        const [invoiceData, contactData] = await Promise.all([
          api.listInvoices(tenantId, {
            type: filterType || void 0,
            status: filterStatus || void 0
          }),
          api.listContacts(tenantId, { active_only: true })
        ]);
        invoices = invoiceData;
        contacts = contactData;
      } catch (err) {
        error = err instanceof Error ? err.message : "Failed to load data";
      } finally {
        isLoading = false;
      }
    }
    async function handleFilter() {
      const tenantId = store_get($$store_subs ??= {}, "$page", page).url.searchParams.get("tenant");
      if (tenantId) {
        loadData(tenantId);
      }
    }
    const typeLabels = {
      SALES: "Sales Invoice",
      PURCHASE: "Purchase Invoice",
      CREDIT_NOTE: "Credit Note"
    };
    const statusLabels = {
      DRAFT: "Draft",
      SENT: "Sent",
      PARTIALLY_PAID: "Partial",
      PAID: "Paid",
      OVERDUE: "Overdue",
      VOIDED: "Voided"
    };
    const statusClass = {
      DRAFT: "badge-draft",
      SENT: "badge-sent",
      PARTIALLY_PAID: "badge-partial",
      PAID: "badge-paid",
      OVERDUE: "badge-overdue",
      VOIDED: "badge-voided"
    };
    function formatCurrency(value) {
      const num = typeof value === "object" && "toFixed" in value ? value.toNumber() : Number(value);
      return new Intl.NumberFormat("et-EE", { style: "currency", currency: "EUR" }).format(num);
    }
    function formatDate(dateStr) {
      return new Date(dateStr).toLocaleDateString("et-EE");
    }
    head("dmf30d", $$renderer2, ($$renderer3) => {
      $$renderer3.title(($$renderer4) => {
        $$renderer4.push(`<title>Invoices - Open Accounting</title>`);
      });
    });
    $$renderer2.push(`<div class="container"><div class="header svelte-dmf30d"><h1 class="svelte-dmf30d">Invoices</h1> <button class="btn btn-primary">+ New Invoice</button></div> <div class="filters card svelte-dmf30d"><div class="filter-row svelte-dmf30d">`);
    $$renderer2.select({ class: "input", value: filterType, onchange: handleFilter }, ($$renderer3) => {
      $$renderer3.option({ value: "" }, ($$renderer4) => {
        $$renderer4.push(`All Types`);
      });
      $$renderer3.option({ value: "SALES" }, ($$renderer4) => {
        $$renderer4.push(`Sales Invoices`);
      });
      $$renderer3.option({ value: "PURCHASE" }, ($$renderer4) => {
        $$renderer4.push(`Purchase Invoices`);
      });
      $$renderer3.option({ value: "CREDIT_NOTE" }, ($$renderer4) => {
        $$renderer4.push(`Credit Notes`);
      });
    });
    $$renderer2.push(` `);
    $$renderer2.select({ class: "input", value: filterStatus, onchange: handleFilter }, ($$renderer3) => {
      $$renderer3.option({ value: "" }, ($$renderer4) => {
        $$renderer4.push(`All Statuses`);
      });
      $$renderer3.option({ value: "DRAFT" }, ($$renderer4) => {
        $$renderer4.push(`Draft`);
      });
      $$renderer3.option({ value: "SENT" }, ($$renderer4) => {
        $$renderer4.push(`Sent`);
      });
      $$renderer3.option({ value: "PARTIALLY_PAID" }, ($$renderer4) => {
        $$renderer4.push(`Partially Paid`);
      });
      $$renderer3.option({ value: "PAID" }, ($$renderer4) => {
        $$renderer4.push(`Paid`);
      });
      $$renderer3.option({ value: "OVERDUE" }, ($$renderer4) => {
        $$renderer4.push(`Overdue`);
      });
    });
    $$renderer2.push(`</div></div> `);
    if (error) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<div class="alert alert-error">${escape_html(error)}</div>`);
    } else {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> `);
    if (isLoading) {
      $$renderer2.push("<!--[-->");
      $$renderer2.push(`<p>Loading invoices...</p>`);
    } else {
      $$renderer2.push("<!--[!-->");
      if (invoices.length === 0) {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<div class="empty-state card svelte-dmf30d"><p>No invoices found. Create your first invoice to get started.</p></div>`);
      } else {
        $$renderer2.push("<!--[!-->");
        $$renderer2.push(`<div class="card"><table class="table"><thead><tr><th>Number</th><th>Type</th><th>Contact</th><th>Issue Date</th><th>Due Date</th><th>Total</th><th>Status</th><th>Actions</th></tr></thead><tbody><!--[-->`);
        const each_array = ensure_array_like(invoices);
        for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
          let invoice = each_array[$$index];
          $$renderer2.push(`<tr><td class="number svelte-dmf30d">${escape_html(invoice.invoice_number)}</td><td>${escape_html(typeLabels[invoice.invoice_type])}</td><td>${escape_html(invoice.contact?.name || "-")}</td><td>${escape_html(formatDate(invoice.issue_date))}</td><td>${escape_html(formatDate(invoice.due_date))}</td><td class="amount svelte-dmf30d">${escape_html(formatCurrency(invoice.total))}</td><td><span${attr_class(`badge ${stringify(statusClass[invoice.status])}`, "svelte-dmf30d")}>${escape_html(statusLabels[invoice.status])}</span></td><td>`);
          if (invoice.status === "DRAFT") {
            $$renderer2.push("<!--[-->");
            $$renderer2.push(`<button class="btn btn-small svelte-dmf30d">Send</button>`);
          } else {
            $$renderer2.push("<!--[!-->");
          }
          $$renderer2.push(`<!--]--></td></tr>`);
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
export {
  _page as default
};
