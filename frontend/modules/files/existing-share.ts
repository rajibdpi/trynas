type Share = { token: string; storage?: string; path: string; paths?: string[]; expiresAt?: string };
export function findExistingShare(shares: Share[], selection: string[], storage = "") {
 const normalize = (paths: string[]) => [...new Set(paths)].filter(path => !paths.some(other => other !== path && (other === "" || path.startsWith(other + "/")))).sort();
 const wanted = normalize(selection);
 return shares.find(share => (share.storage || "") === storage && (!share.expiresAt || Date.parse(share.expiresAt) > Date.now()) && JSON.stringify(normalize(share.paths?.length ? share.paths : [share.path])) === JSON.stringify(wanted));
}
