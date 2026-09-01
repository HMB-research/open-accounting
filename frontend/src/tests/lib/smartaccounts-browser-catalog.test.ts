import { describe, expect, it } from "vitest";
import {
  canonicalizeSmartAccountsBrowserCatalog,
  sha256Hex,
} from "$lib/utils/smartaccounts-browser-catalog";

const companies = [
  {
    source_company_id: "sa-browser-v1-9876",
    source_company_name: "Company B",
  },
  {
    source_company_id: "sa-browser-v1-1234",
    source_company_name: "Company A",
  },
];

describe("SmartAccounts browser catalog canonicalization", () => {
  it("sorts companies and produces the server-compatible digest without Web Crypto", async () => {
    const catalog = await canonicalizeSmartAccountsBrowserCatalog(
      companies,
      undefined,
    );

    expect(
      catalog.companies.map((company) => company.source_company_id),
    ).toEqual(["sa-browser-v1-1234", "sa-browser-v1-9876"]);
    expect(catalog.sha256).toBe(
      "0e5c2dcf6454d4bc6bfc228e9e43e0d3fb45e0ef26386319e2805ef6c1ef60dc",
    );
  });

  it("matches the standard SHA-256 empty-message vector", () => {
    expect(sha256Hex(new Uint8Array())).toBe(
      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    );
  });
});
