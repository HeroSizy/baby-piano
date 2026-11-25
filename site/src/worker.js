// mashpiano.herosizy.dev. Everything is the static page in public/,
// except:
//   /download/<platform>  302 to that installer of the latest release
//   /api/latest           the latest release's version and installers
// The release is asked of GitHub at most every 10 minutes (Cache API).

const REPO = "HeroSizy/baby-piano";
const RELEASES = `https://github.com/${REPO}/releases/latest`;

// The end of each installer's file name, as the build workflow names it
// (mash-piano-1.0.12-macos-arm64.dmg).
const PLATFORMS = {
  "mac-arm64": "-macos-arm64.dmg",
  "mac-x64": "-macos-x64.dmg",
  "windows-x64": "-windows-x64-setup.exe",
  "windows-arm64": "-windows-arm64-setup.exe",
  android: "-android-arm64.apk",
};

async function latest(ctx) {
  const key = new Request("https://mashpiano.herosizy.dev/__cache/latest");
  const cache = caches.default;
  const hit = await cache.match(key);
  if (hit) return hit.json();

  const res = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`, {
    headers: {
      accept: "application/vnd.github+json",
      "user-agent": "mashpiano-site",
    },
  });
  if (!res.ok) throw new Error(`GitHub ${res.status}`);
  const rel = await res.json();
  const assets = {};
  for (const [platform, suffix] of Object.entries(PLATFORMS)) {
    const a = rel.assets.find((a) => a.name.endsWith(suffix));
    if (a) assets[platform] = { url: a.browser_download_url, name: a.name, size: a.size };
  }
  const data = {
    version: rel.tag_name.replace(/^v/, ""),
    published: rel.published_at,
    url: rel.html_url,
    assets,
  };
  ctx.waitUntil(
    cache.put(
      key,
      new Response(JSON.stringify(data), {
        headers: { "content-type": "application/json", "cache-control": "max-age=600" },
      }),
    ),
  );
  return data;
}

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);

    if (url.pathname === "/api/latest") {
      try {
        return Response.json(await latest(ctx), {
          headers: { "cache-control": "public, max-age=300" },
        });
      } catch (e) {
        return Response.json({ error: String(e.message || e) }, { status: 502 });
      }
    }

    const m = url.pathname.match(/^\/download\/([a-z0-9-]+)\/?$/);
    if (m) {
      let to = RELEASES;
      if (PLATFORMS[m[1]]) {
        try {
          to = (await latest(ctx)).assets[m[1]]?.url || RELEASES;
        } catch {
          // GitHub unreachable or rate limited: the release page still works.
        }
      }
      return new Response(null, {
        status: 302,
        headers: { location: to, "cache-control": "no-store" },
      });
    }

    return env.ASSETS.fetch(request);
  },
};
