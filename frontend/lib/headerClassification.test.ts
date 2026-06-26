import { describe, it, expect } from "vitest";
import { isSecurityHeader, filterSecurityHeaders, changedSecurityHeaders } from "./headerClassification";

describe("isSecurityHeader", () => {
  it("recognizes security headers regardless of case", () => {
    expect(isSecurityHeader("Content-Security-Policy")).toBe(true);
    expect(isSecurityHeader("strict-transport-security")).toBe(true);
    expect(isSecurityHeader("X-Frame-Options")).toBe(true);
  });

  it("treats set-cookie as security-relevant (cookie flags matter)", () => {
    expect(isSecurityHeader("Set-Cookie")).toBe(true);
  });

  it("recognizes CORS access-control headers by prefix", () => {
    expect(isSecurityHeader("Access-Control-Allow-Origin")).toBe(true);
  });

  it("treats churn headers as noise", () => {
    expect(isSecurityHeader("date")).toBe(false);
    expect(isSecurityHeader("etag")).toBe(false);
    expect(isSecurityHeader("x-request-id")).toBe(false);
    expect(isSecurityHeader("paypal-debug-id")).toBe(false);
  });
});

describe("filterSecurityHeaders", () => {
  it("keeps only the security-relevant headers", () => {
    const filtered = filterSecurityHeaders({
      "content-security-policy": ["default-src 'self'"],
      date: ["Fri, 26 Jun 2026 11:15:51 GMT"],
      etag: ['W/"abc"'],
    });
    expect(Object.keys(filtered)).toEqual(["content-security-policy"]);
  });
});

describe("changedSecurityHeaders", () => {
  it("reports a security header whose value changed", () => {
    const base = { "content-security-policy": ["default-src 'self'"], date: ["a"] };
    const head = { "content-security-policy": ["default-src 'self'; script-src 'unsafe-inline'"], date: ["b"] };
    expect(changedSecurityHeaders(base, head)).toEqual(["content-security-policy"]);
  });

  it("ignores noise headers that churn every fetch", () => {
    const base = { date: ["a"], etag: ['W/"1"'], "x-frame-options": ["DENY"] };
    const head = { date: ["b"], etag: ['W/"2"'], "x-frame-options": ["DENY"] };
    expect(changedSecurityHeaders(base, head)).toEqual([]);
  });

  it("detects an added or removed security header", () => {
    expect(changedSecurityHeaders({}, { "strict-transport-security": ["max-age=31536000"] })).toEqual([
      "strict-transport-security",
    ]);
  });
});
