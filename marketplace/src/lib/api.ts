const API = "";

export interface SearchResult {
  name: string;
  type: "plugin" | "profile";
  version: string;
  description: string;
  category?: string;
  runtime?: string;
  tools?: string[];
  capabilities?: string[];
  model?: string;
  extends?: string;
  downloads: number;
}

export interface PackageDetail {
  name: string;
  type: string;
  version: string;
  description: string;
  category?: string;
  runtime?: string;
  tools?: string[];
  dependencies?: string[];
  capabilities?: string[];
  accepts?: string;
  returns?: string;
  extends?: string;
  mode?: string;
  model?: string;
  provider?: string;
  downloads: number;
  readme?: string;
}

export async function search(
  q?: string,
  type?: string,
  category?: string,
  sort?: string
): Promise<{ results: SearchResult[]; count: number }> {
  const params = new URLSearchParams();
  if (q) params.set("q", q);
  if (type) params.set("type", type);
  if (category) params.set("category", category);
  if (sort) params.set("sort", sort);
  const resp = await fetch(`${API}/v1/search?${params}`);
  return resp.json();
}

export async function getPluginDetail(name: string): Promise<PackageDetail> {
  const resp = await fetch(`${API}/v1/packages/${name}/detail.json`);
  return resp.json();
}

export async function getProfileDetail(name: string): Promise<PackageDetail> {
  const resp = await fetch(`${API}/v1/profiles/${name}/detail.json`);
  return resp.json();
}

export async function getHealth(): Promise<{
  ok: boolean;
  packages: number;
  profiles: number;
}> {
  const resp = await fetch(`${API}/healthz`);
  return resp.json();
}
