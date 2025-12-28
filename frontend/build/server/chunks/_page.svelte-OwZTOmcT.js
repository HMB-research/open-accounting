import { U as head, V as attr } from './index2-VjzSvL4X.js';
import 'decimal.js';
import { e as escape_html } from './context-Cv9QAF3V.js';
import './false-CRHihH2U.js';

function _page($$renderer, $$props) {
  $$renderer.component(($$renderer2) => {
    let email = "";
    let password = "";
    let isLoading = false;
    head("1x05zx6", $$renderer2, ($$renderer3) => {
      $$renderer3.title(($$renderer4) => {
        $$renderer4.push(`<title>${escape_html("Login")} - Open Accounting</title>`);
      });
    });
    $$renderer2.push(`<div class="login-page svelte-1x05zx6"><div class="login-card card svelte-1x05zx6"><h1 class="svelte-1x05zx6">${escape_html("Welcome Back")}</h1> <p class="subtitle svelte-1x05zx6">${escape_html("Sign in to your account")}</p> `);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <form>`);
    {
      $$renderer2.push("<!--[!-->");
    }
    $$renderer2.push(`<!--]--> <div class="form-group"><label class="label" for="email">Email</label> <input class="input" type="email" id="email"${attr("value", email)} required placeholder="you@example.com"/></div> <div class="form-group"><label class="label" for="password">Password</label> <input class="input" type="password" id="password"${attr("value", password)} required minlength="8" placeholder="Min 8 characters"/></div> <button class="btn btn-primary btn-full svelte-1x05zx6" type="submit"${attr("disabled", isLoading, true)}>`);
    {
      $$renderer2.push("<!--[!-->");
      {
        $$renderer2.push("<!--[!-->");
        $$renderer2.push(`Sign In`);
      }
      $$renderer2.push(`<!--]-->`);
    }
    $$renderer2.push(`<!--]--></button></form> <p class="toggle-mode svelte-1x05zx6">`);
    {
      $$renderer2.push("<!--[!-->");
      $$renderer2.push(`Don't have an account? <button class="link-btn svelte-1x05zx6" type="button">Create one</button>`);
    }
    $$renderer2.push(`<!--]--></p></div></div>`);
  });
}

export { _page as default };
//# sourceMappingURL=_page.svelte-OwZTOmcT.js.map
