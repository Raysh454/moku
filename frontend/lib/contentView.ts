import type { Snapshot } from "../types/project";

export type SnapshotViewKind = "html" | "image" | "json" | "text" | "directory" | "binary";

export type SnapshotContentInfo = {
  contentType: string;
  viewKind: SnapshotViewKind;
  textBody: string;
  rawBody: string;
  bodyEncoding: "text" | "base64";
  imageSrc?: string;
};

const BASE64_PATTERN = /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/;

const stripCharset = (value: string): string => value.split(";")[0]?.trim().toLowerCase() || "";

export const readHeaderValue = (headers: Record<string, string[]>, name: string): string => {
  const target = name.toLowerCase();
  for (const [key, values] of Object.entries(headers || {})) {
    if (key.toLowerCase() === target && values.length > 0) return values[0] || "";
  }
  return "";
};

export const isLikelyBase64 = (value: string): boolean => {
  if (!value || value.length < 16 || value.length % 4 !== 0) return false;
  if (!BASE64_PATTERN.test(value)) return false;
  return true;
};

const decodeBase64Safely = (value: string): string => {
  try {
    const binary = atob(value);
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
    return new TextDecoder("utf-8", { fatal: false }).decode(bytes);
  } catch {
    return "";
  }
};

const looksLikeHtml = (value: string): boolean => {
  const probe = value.slice(0, 512).toLowerCase();
  return probe.includes("<!doctype") || probe.includes("<html") || probe.includes("<head") || probe.includes("<body");
};

const looksLikeJson = (value: string): boolean => {
  const trimmed = value.trim();
  return trimmed.startsWith("{") || trimmed.startsWith("[");
};

const isMostlyText = (value: string): boolean => {
  if (!value) return false;
  const sample = value.slice(0, 2048);
  let printable = 0;
  for (let index = 0; index < sample.length; index += 1) {
    const code = sample.charCodeAt(index);
    if (code === 9 || code === 10 || code === 13 || (code >= 32 && code <= 126) || code > 159) {
      printable += 1;
    }
  }
  return printable / sample.length > 0.9;
};

const isDirectoryListingHtml = (html: string): boolean => {
  const probe = html.slice(0, 4000).toLowerCase();
  return probe.includes("index of /") && probe.includes("<a href=");
};

const inferViewKind = (contentType: string, textBody: string): SnapshotViewKind => {
  if (contentType.startsWith("image/")) return "image";
  if (contentType.includes("json")) return "json";
  if (contentType.includes("html") || looksLikeHtml(textBody)) {
    return isDirectoryListingHtml(textBody) ? "directory" : "html";
  }
  if (contentType.startsWith("text/")) return "text";
  if (looksLikeJson(textBody)) return "json";
  if (isMostlyText(textBody)) return "text";
  return "binary";
};

export const getSnapshotContentInfo = (snapshot: Snapshot | null | undefined): SnapshotContentInfo => {
  const rawBody = snapshot?.body || "";
  const headerContentType = stripCharset(readHeaderValue(snapshot?.headers || {}, "content-type"));
  const contentType = snapshot?.metadata.contentType || headerContentType || "application/octet-stream";

  if (!rawBody) {
    return {
      contentType,
      viewKind: inferViewKind(contentType, ""),
      textBody: "",
      rawBody: "",
      bodyEncoding: snapshot?.metadata.bodyEncoding || "text",
    };
  }

  const looksEncoded = snapshot?.metadata.bodyEncoding === "base64" || isLikelyBase64(rawBody);
  const shouldDecodeToText =
    contentType.startsWith("text/") ||
    contentType.includes("json") ||
    contentType.includes("xml") ||
    contentType.includes("javascript") ||
    contentType.includes("html") ||
    contentType.includes("svg");

  const decodedBody = looksEncoded ? decodeBase64Safely(rawBody) : rawBody;
  const textBody = shouldDecodeToText ? decodedBody || rawBody : decodedBody;
  const bodyEncoding: "text" | "base64" =
    snapshot?.metadata.bodyEncoding || (looksEncoded && !shouldDecodeToText ? "base64" : "text");
  const viewKind = snapshot?.metadata.viewKind || inferViewKind(contentType, textBody);

  let imageSrc: string | undefined;
  if (viewKind === "image") {
    if (bodyEncoding === "base64") {
      imageSrc = `data:${contentType};base64,${rawBody}`;
    } else if (isLikelyBase64(rawBody)) {
      imageSrc = `data:${contentType};base64,${rawBody}`;
    }
  }

  return {
    contentType,
    viewKind,
    textBody,
    rawBody,
    bodyEncoding,
    imageSrc,
  };
};
