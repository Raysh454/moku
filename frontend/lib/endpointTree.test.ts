import { ancestorFolderPaths, buildEndpointTree, flattenEndpointLeaves, INDEX_LEAF, type TreeNode } from "./endpointTree";
import type { Domain, Endpoint } from "../types/project";
import type { SecurityDiffOverviewEntry } from "../src/api/types";

function makeEndpoint(id: string, path: string, url = `https://example.com${path}`): Endpoint {
  return {
    id,
    url,
    canonicalUrl: url,
    path,
    status: "fetched",
    source: "spider",
    meta: "",
    lastFetchedVersion: "",
    snapshots: [],
  };
}

function makeDomain(endpoints: Endpoint[]): Domain {
  return { id: "d1", slug: "example", hostname: "example.com", origin: "https://example.com", endpoints, versions: [] };
}

function childNamed(nodes: TreeNode[], name: string): TreeNode | undefined {
  return nodes.find((node) => node.name === name);
}

describe("buildEndpointTree", () => {
  it("nests endpoints by url path segments", () => {
    const tree = buildEndpointTree(
      makeDomain([makeEndpoint("a", "/"), makeEndpoint("b", "/login"), makeEndpoint("c", "/admin/users")]),
    );

    expect(childNamed(tree, "login")?.kind).toBe("endpoint");
    const admin = childNamed(tree, "admin");
    expect(admin?.kind).toBe("folder");
    expect(childNamed(admin?.children ?? [], "users")?.endpointId).toBe("c");
  });

  it("treats a page that is also a folder as an index leaf", () => {
    const tree = buildEndpointTree(makeDomain([makeEndpoint("a", "/admin"), makeEndpoint("b", "/admin/users")]));

    const admin = childNamed(tree, "admin");
    expect(admin?.kind).toBe("folder");
    const indexLeaf = childNamed(admin?.children ?? [], "/");
    expect(indexLeaf?.kind).toBe("endpoint");
    expect(indexLeaf?.endpointId).toBe("a");
  });

  it("marks a leaf modified when its security score regressed", () => {
    const overview: SecurityDiffOverviewEntry[] = [{ url: "/login", score_base: 1, score_head: 4, score_delta: 3 }];
    const tree = buildEndpointTree(makeDomain([makeEndpoint("b", "/login")]), overview);
    expect(childNamed(tree, "login")?.status).toBe("modified");
  });

  it("marks a leaf added when it is absent from the base version", () => {
    const overview: SecurityDiffOverviewEntry[] = [{ url: "/new", score_head: 2 }];
    const tree = buildEndpointTree(makeDomain([makeEndpoint("n", "/new")]), overview);
    expect(childNamed(tree, "new")?.status).toBe("added");
  });

  it("rolls up has-changes to ancestor folders", () => {
    const overview: SecurityDiffOverviewEntry[] = [{ url: "/admin/users", score_delta: 5 }];
    const tree = buildEndpointTree(makeDomain([makeEndpoint("c", "/admin/users")]), overview);
    const admin = childNamed(tree, "admin");
    expect(admin?.hasChanges).toBe(true);
  });

  it("leaves folders unchanged when no descendant changed", () => {
    const tree = buildEndpointTree(makeDomain([makeEndpoint("c", "/admin/users")]));
    expect(childNamed(tree, "admin")?.hasChanges).toBe(false);
  });

  it("falls back to the url pathname when the endpoint path is empty", () => {
    const tree = buildEndpointTree(makeDomain([makeEndpoint("z", "", "https://example.com/reports/q3")]));
    const reports = childNamed(tree, "reports");
    expect(childNamed(reports?.children ?? [], "q3")?.endpointId).toBe("z");
  });
});

describe("flattenEndpointLeaves", () => {
  it("returns one leaf per endpoint with its tree path", () => {
    const tree = buildEndpointTree(makeDomain([makeEndpoint("a", "/"), makeEndpoint("c", "/admin/users")]));
    const leaves = flattenEndpointLeaves(tree);
    expect(leaves.map((leaf) => leaf.endpointId).sort()).toEqual(["a", "c"]);
    expect(leaves.find((leaf) => leaf.endpointId === "c")?.path).toBe("admin/users");
  });
});

describe("exposure delta", () => {
  it("attaches the overview exposure_delta to the endpoint leaf", () => {
    const overview: SecurityDiffOverviewEntry[] = [{ url: "/login", exposure_delta: 0.4 }];
    const tree = buildEndpointTree(makeDomain([makeEndpoint("b", "/login")]), overview);
    expect(childNamed(tree, "login")?.exposureDelta).toBe(0.4);
  });

  it("flattens exposure delta onto leaves for the tree adapter", () => {
    const overview: SecurityDiffOverviewEntry[] = [{ url: "/login", exposure_delta: 0.4 }];
    const leaves = flattenEndpointLeaves(buildEndpointTree(makeDomain([makeEndpoint("b", "/login")]), overview));
    expect(leaves[0]?.exposureDelta).toBe(0.4);
  });

  it("orders sibling endpoints by exposure delta, largest regression first", () => {
    const overview: SecurityDiffOverviewEntry[] = [
      { url: "/a", exposure_delta: 0.1 },
      { url: "/b", exposure_delta: 0.9 },
      { url: "/c", exposure_delta: 0.5 },
    ];
    const tree = buildEndpointTree(
      makeDomain([makeEndpoint("a", "/a"), makeEndpoint("b", "/b"), makeEndpoint("c", "/c")]),
      overview,
    );
    expect(tree.map((node) => node.name)).toEqual(["b", "c", "a"]);
  });
});

describe("ancestorFolderPaths", () => {
  it("lists every parent folder of a nested leaf, root first", () => {
    expect(ancestorFolderPaths("auth/createchallenge/recaptchav3.js")).toEqual(["auth", "auth/createchallenge"]);
  });

  it("returns no ancestors for a top-level leaf", () => {
    expect(ancestorFolderPaths("login")).toEqual([]);
  });

  it("treats an index leaf's containing folder as its only ancestor", () => {
    expect(ancestorFolderPaths(`admin/${INDEX_LEAF}`)).toEqual(["admin"]);
  });
});
