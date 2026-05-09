export async function request(url: string, options: RequestInit = {}) {
  const res = await fetch(url, {
    credentials: "include",
    ...options,
  });

  if (res.ok) {
    const ct = res.headers.get("content-type") || "";
    if (ct.includes("application/json")) {
      return res.json();
    }
    return res.text();
  }

  const text = await res.text();
  const err: any = new Error(text || res.statusText || "Request failed");
  err.status = res.status;
  throw err;
}
