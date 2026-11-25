#!/usr/bin/env python3
"""Uploads the App Store listing translations in store/*.yaml to App Store Connect.

Each store/<lang>.yaml holds one language's subtitle, keywords, promotional
text, description and what's new, and the App Store locales it goes to.
They go to the version being prepared (one is created, named after the
newest release tag, when none is), and the subtitle to the app info that
goes with it. The name stays Mash Piano, and the support, marketing and
privacy links are copied from English (U.S.). Neither English nor
Simplified Chinese is touched. Screenshots are left to fall back to
English's.

Needs APPLE_API_KEY_P8 (the key's text), APPLE_API_KEY_ID and
APPLE_API_ISSUER. --dry-run reads App Store Connect and prints what it
would change without changing anything.
"""
import argparse, glob, json, os, subprocess, sys, time, urllib.request, urllib.error

import jwt  # PyJWT
import yaml

APP_ID = "6837722350"
BASE = "https://api.appstoreconnect.apple.com"
EDITABLE = {"PREPARE_FOR_SUBMISSION", "DEVELOPER_REJECTED", "REJECTED", "METADATA_REJECTED", "INVALID_BINARY"}
LIVE = {"READY_FOR_SALE", "READY_FOR_DISTRIBUTION", "REPLACED_WITH_NEW_VERSION", "REMOVED_FROM_SALE", "DEVELOPER_REMOVED_FROM_SALE"}


def token():
    key = os.environ["APPLE_API_KEY_P8"]
    if os.path.isfile(key):
        key = open(key).read()
    now = int(time.time())
    return jwt.encode({"iss": os.environ["APPLE_API_ISSUER"], "iat": now, "exp": now + 1200, "aud": "appstoreconnect-v1"},
                      key, algorithm="ES256", headers={"kid": os.environ["APPLE_API_KEY_ID"], "typ": "JWT"})


TOKEN = None


def call(method, path, body=None):
    req = urllib.request.Request(BASE + path, method=method, data=json.dumps(body).encode() if body else None,
                                 headers={"Authorization": "Bearer " + TOKEN, "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req) as r:
            raw = r.read()
            return json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        sys.exit(f"{method} {path}: {e.code} {e.read().decode()}")


def locs(path):
    return {d["attributes"]["locale"]: d for d in call("GET", path + "?limit=200")["data"]}


def main():
    global TOKEN
    ap = argparse.ArgumentParser()
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--version", help="the version to create when none is being prepared (default: the newest release tag)")
    a = ap.parse_args()
    TOKEN = token()
    dry = a.dry_run

    versions = call("GET", f"/v1/apps/{APP_ID}/appStoreVersions?filter[platform]=IOS&limit=20")["data"]
    for v in versions:
        print("version", v["attributes"]["versionString"], v["attributes"]["appStoreState"])
    editable = [v for v in versions if v["attributes"]["appStoreState"] in EDITABLE]
    if editable:
        version = editable[0]
    else:
        name = a.version or subprocess.check_output(["git", "describe", "--tags", "--abbrev=0", "--match", "v*"], text=True).strip().lstrip("v")
        print("creating version", name)
        if dry:
            version = None
        else:
            version = call("POST", "/v1/appStoreVersions", {"data": {"type": "appStoreVersions",
                "attributes": {"platform": "IOS", "versionString": name},
                "relationships": {"app": {"data": {"type": "apps", "id": APP_ID}}}}})["data"]
    if version:
        print("listing goes to version", version["attributes"]["versionString"], version["attributes"]["appStoreState"])

    infos = call("GET", f"/v1/apps/{APP_ID}/appInfos")["data"]
    for i in infos:
        print("app info", i["id"], i["attributes"].get("state"), i["attributes"].get("appStoreState"))
    info = next((i for i in infos if i["attributes"].get("appStoreState") not in LIVE), None)
    live_info = next((i for i in infos if i["attributes"].get("appStoreState") in LIVE), infos[0])

    en_info = locs(f"/v1/appInfos/{live_info['id']}/appInfoLocalizations")["en-US"]["attributes"]
    privacy = en_info.get("privacyPolicyUrl")
    ver_for_links = version or next(v for v in versions if v["attributes"]["appStoreState"] in LIVE)
    en_ver = locs(f"/v1/appStoreVersions/{ver_for_links['id']}/appStoreVersionLocalizations")["en-US"]["attributes"]
    links = {k: en_ver[k] for k in ("supportUrl", "marketingUrl") if en_ver.get(k)}
    print("links", privacy, links)

    have_info = locs(f"/v1/appInfos/{info['id']}/appInfoLocalizations") if info else {}
    have_ver = locs(f"/v1/appStoreVersions/{version['id']}/appStoreVersionLocalizations") if version else {}

    root = os.path.join(os.path.dirname(__file__), "..", "store")
    for f in sorted(glob.glob(os.path.join(root, "*.yaml"))):
        t = yaml.safe_load(open(f))
        for loc in t["locales"]:
            ia = {"name": "Mash Piano", "subtitle": t["subtitle"]}
            if privacy:
                ia["privacyPolicyUrl"] = privacy
            va = dict(links, description=t["description"].strip(), keywords=t["keywords"],
                      promotionalText=t["promo"], whatsNew=t["whatsNew"])
            print(f"{loc}: app info {'update' if loc in have_info else 'add'}, version {'update' if loc in have_ver else 'add'}")
            if dry:
                continue
            if not info:
                sys.exit("no app info is being prepared")
            if loc in have_info:
                call("PATCH", "/v1/appInfoLocalizations/" + have_info[loc]["id"],
                     {"data": {"type": "appInfoLocalizations", "id": have_info[loc]["id"], "attributes": {"subtitle": t["subtitle"]}}})
            else:
                call("POST", "/v1/appInfoLocalizations", {"data": {"type": "appInfoLocalizations",
                     "attributes": dict(ia, locale=loc), "relationships": {"appInfo": {"data": {"type": "appInfos", "id": info["id"]}}}}})
            if loc in have_ver:
                call("PATCH", "/v1/appStoreVersionLocalizations/" + have_ver[loc]["id"],
                     {"data": {"type": "appStoreVersionLocalizations", "id": have_ver[loc]["id"], "attributes": va}})
            else:
                call("POST", "/v1/appStoreVersionLocalizations", {"data": {"type": "appStoreVersionLocalizations",
                     "attributes": dict(va, locale=loc), "relationships": {"appStoreVersion": {"data": {"type": "appStoreVersions", "id": version["id"]}}}}})
    print("done" if not dry else "dry run: nothing changed")


main()
