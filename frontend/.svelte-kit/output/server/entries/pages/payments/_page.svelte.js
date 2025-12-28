import { U as head, W as store_get, X as unsubscribe_stores, Y as ensure_array_like, Z as attr_class, _ as stringify } from "../../../chunks/index2.js";
import { p as page } from "../../../chunks/stores.js";
import { a as api } from "../../../chunks/api.js";
import Decimal from "decimal.js";
import { e as escape_html } from "../../../chunks/context.js";
function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    var $$store_subs;
    let payments = [];
    let contacts = [];
    let unpaidInvoices = [];
    let isLoading = true;
    let error = "";
    let filterType = "";
    (/* @__PURE__ */ new Date()).toISOString().split("T")[0];
    async function loadData(tenantId) {
      isLoading = true;
      error = "";
      try {
        const [paymentData, contactData, invoiceData] = await Promise.all([
          api.listPayments(tenantId, { type: filterType || void 0 }),
          api.listContacts(tenantId, { active_only: true }),
          api.listInvoices(tenantId, { status: "SENT" })
        ]);
        payments = paymentData;
        contacts = contactData;
        unpaidInvoices = invoiceData;
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
    const typeLabels = { RECEIVED: "Payment Received", MADE: "Payment Made" };
    const typeBadgeClass = { RECEIVED: "badge-received", MADE: "badge-made" };
    const methodLabels = {
      BANK_TRANSFER: "Bank Transfer",
      CASH: "Cash",
      CARD: "Card",
      OTHER: "Other"
    };
    function formatCurrency(value) {
      const num = typeof value === "object" && "toFixed" in value ? value.toNumber() : Number(value);
      return new Intl.NumberFormat("et-EE", { style: "currency", currency: "EUR" }).format(num);
    }
    function formatDate(dateStr) {
      return new Date(dateStr).toLocaleDateString("et-EE");
    }
    function getContactName(contactId) {
      if (!contactId) return "-";
      const contact = contacts.find((c) => c.id === contactId);
      return contact?.name || "-";
    }
    function getUnallocatedAmount(payment) {
      const total = payment.amount;
      const allocated = payment.allocations.reduce((sum, a) => sum.plus(a.amount), new Decimal(0));
      return new Decimal(total).minus(allocated);
    }
    head("zxnq2c", $$renderer2, ($$renderer3) => {
      $$renderer3.title(($$renderer4) => {
        $$renderer4.push(`<title>Payments - Open Accounting</title>`);
      });
    });
    $$renderer2.push(`<div class="container"><div class="header svelte-zxnq2c"><h1 class="svelte-zxnq2c">Payments</h1> <button class="btn btn-primary">+ New Payment</button></div> <div class="filters card svelte-zxnq2c"><div class="filter-row svelte-zxnq2c">`);
    $$renderer2.select({ class: "input", value: filterType, onchange: handleFilter }, ($$renderer3) => {
      $$renderer3.option({ value: "" }, ($$renderer4) => {
        $$renderer4.push(`All Payments`);
      });
      $$renderer3.option({ value: "RECEIVED" }, ($$renderer4) => {
        $$renderer4.push(`Received`);
      });
      $$renderer3.option({ value: "MADE" }, ($$renderer4) => {
        $$renderer4.push(`Made`);
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
      $$renderer2.push(`<p>Loading payments...</p>`);
    } else {
      $$renderer2.push("<!--[!-->");
      if (payments.length === 0) {
        $$renderer2.push("<!--[-->");
        $$renderer2.push(`<div class="empty-state card svelte-zxnq2c"><p>No payments found. Record your first payment to get started.</p></div>`);
      } else {
        $$renderer2.push("<!--[!-->");
        $$renderer2.push(`<div class="card"><table class="table"><thead><tr><th>Number</th><th>Type</th><th>Contact</th><th>Date</th><th>Method</th><th>Amount</th><th>Unallocated</th><th>Reference</th></tr></thead><tbody><!--[-->`);
        const each_array = ensure_array_like(payments);
        for (let $$index = 0, $$length = each_array.length; $$index < $$length; $$index++) {
          let payment = each_array[$$index];
          const unallocated = getUnallocatedAmount(payment);
          $$renderer2.push(`<tr><td class="number svelte-zxnq2c">${escape_html(payment.payment_number)}</td><td><span${attr_class(`badge ${stringify(typeBadgeClass[payment.payment_type])}`, "svelte-zxnq2c")}>${escape_html(typeLabels[payment.payment_type])}</span></td><td>${escape_html(getContactName(payment.contact_id))}</td><td>${escape_html(formatDate(payment.payment_date))}</td><td>${escape_html(methodLabels[payment.payment_method || "OTHER"] || payment.payment_method)}</td><td class="amount svelte-zxnq2c">${escape_html(formatCurrency(payment.amount))}</td><td${attr_class("amount svelte-zxnq2c", void 0, { "unallocated-warning": unallocated.greaterThan(0) })}>${escape_html(formatCurrency(unallocated))}</td><td class="reference svelte-zxnq2c">${escape_html(payment.reference || "-")}</td></tr>`);
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
